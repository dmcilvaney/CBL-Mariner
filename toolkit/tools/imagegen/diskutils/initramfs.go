// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Utility to encrypt disks and partitions

package diskutils

import (
	"bytes"
	"io"
	"os"

	"github.com/cavaliercoder/go-cpio"
	"github.com/klauspost/pgzip"
	"microsoft.com/pkggen/internal/logger"
)

// InitramfsMount represented an editable initramfs
type InitramfsMount struct {
	pgzWriter           *pgzip.Writer
	cpioWriter          *cpio.Writer
	outputBuffer        *bytes.Buffer
	initramfsOutputFile *os.File
}

// CreateInitramfs creates a new initramfs
func CreateInitramfs(initramfsPath string) (initramfs InitramfsMount, err error) {
	// Initramfs traditionally is -rw-------
	const initramfsModeBits = os.FileMode(0600)
	// Caller must use InitramfsMount.Close() to clean up the outputs
	initramfs.outputBuffer = new(bytes.Buffer)
	initramfs.pgzWriter = pgzip.NewWriter(initramfs.outputBuffer)
	initramfs.cpioWriter = cpio.NewWriter(initramfs.pgzWriter)

	initramfs.initramfsOutputFile, err = os.OpenFile(initramfsPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, initramfsModeBits)

	return
}

// OpenInitramfs makes an existing initramfs editable
func OpenInitramfs(initramfsPath string) (initramfs InitramfsMount, err error) {
	inputFile, err := os.Open(initramfsPath)
	if err != nil {
		return
	}
	defer inputFile.Close()

	gzReader, err := pgzip.NewReader(inputFile)
	if err != nil {
		return
	}
	defer gzReader.Close()
	cpioReader := cpio.NewReader(gzReader)
	//cpio.Reader has no Close() function to defer

	// Caller must use InitramfsMount.Close() to clean up the outputs
	initramfs.outputBuffer = new(bytes.Buffer)
	initramfs.pgzWriter = pgzip.NewWriter(initramfs.outputBuffer)
	initramfs.cpioWriter = cpio.NewWriter(initramfs.pgzWriter)

	var bytesIO int64
	var nextFileHeader *cpio.Header
	for {
		var linkPayload []byte

		nextFileHeader, err = cpioReader.Next()
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}

		// For a given symlink, generate a payload that contains the read value of the link.
		// The payload should be written after the header.
		// e.g. A link from (/bin -> /usr/bin) would have a payload of "/usr/bin").
		isLink := (nextFileHeader.Mode&cpio.ModeSymlink != 0)
		if isLink {
			linkPayload = []byte(nextFileHeader.Linkname)
			nextFileHeader.Size = int64(len(linkPayload))
		}

		err = initramfs.cpioWriter.WriteHeader(nextFileHeader)
		if err != nil {
			return
		}

		if isLink {
			var bytesWrittenInt int

			// Write returns an int, cast it to an int64 afterwards
			logger.Log.Infof("Creating link %s -> %s", nextFileHeader.Name, nextFileHeader.Linkname)
			bytesWrittenInt, err = initramfs.cpioWriter.Write(linkPayload)
			bytesIO = int64(bytesWrittenInt)

		} else {
			bytesIO, err = io.Copy(initramfs.cpioWriter, cpioReader)
		}

		if err != nil {
			return
		}

		logger.Log.Tracef("File %s caused %d bytes to be transfered to new archive", nextFileHeader.Name, bytesIO)
		logger.Log.Tracef("Buffer unread length: %d", initramfs.outputBuffer.Len())
	}

	inputFile.Close()

	fileInfo, err := os.Stat(initramfsPath)
	if err != nil {
		return
	}

	// We can't edit a CPIO archive in place, completely overwrite the file with truncate
	// The output buffer in memory will be used to re-create the initramfs.
	initramfs.initramfsOutputFile, err = os.OpenFile(initramfsPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fileInfo.Mode())

	return
}

// Close flushes the archives and closes all initramfs resources
func (i *InitramfsMount) Close() (err error) {
	var bytesIO int

	logger.Log.Infof("Closing initramfs file '%s'", i.initramfsOutputFile.Name())

	// Defer close calls to make sure we handle any errors, failing to
	// close the file means we can't close the install root.
	defer i.initramfsOutputFile.Close()
	defer i.pgzWriter.Close()
	defer i.cpioWriter.Close()

	err = i.cpioWriter.Close()
	if err != nil {
		logger.Log.Errorf("Failed to close initramfs: '%s'", err.Error())
		return
	}
	err = i.pgzWriter.Close()
	if err != nil {
		logger.Log.Errorf("Failed to close initramfs: '%s'", err.Error())
		return
	}

	logger.Log.Debugf("Writing %d bytes to file", i.outputBuffer.Len())
	bytesIO, err = i.initramfsOutputFile.Write(i.outputBuffer.Bytes())
	if err != nil {
		logger.Log.Errorf("Failed to write initramfs file: '%s'", err.Error())
		//runtime.Breakpoint()
		return
	}
	logger.Log.Infof("Bytes writen to file: %d", bytesIO)

	// Explicit call to fsync, archive corruption was occuring occasionally otherwise.
	err = i.initramfsOutputFile.Sync()
	if err != nil {
		logger.Log.Errorf("Failed to sync initramfs file: '%s'", err.Error())
		return
	}

	err = i.initramfsOutputFile.Close()
	if err != nil {
		logger.Log.Errorf("Failed to close initramfs: '%s'", err.Error())
		return
	}
	return
}

// AddFileToInitramfs places a single file in the initramfs at the destination path.
// - sourcePath: Path to file which is to be adde
// - destPath: Final destination in the initramfs
func (i *InitramfsMount) AddFileToInitramfs(sourcePath, destPath string) (err error) {
	var bytesIO int64
	fileInfo, err := os.Stat(sourcePath)
	file, err := os.Open(sourcePath)
	if err != nil {
		return
	}
	defer file.Close()

	// Symlinks need to be resolved to their target file to be added to the cpio archive.
	var link string
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(sourcePath)
		if err != nil {
			return
		}

		logger.Log.Debugf("--> Adding link: (%s) -> (%s)", sourcePath, link)
	}

	// Convert the OS header into a CPIO header
	header, err := cpio.FileInfoHeader(fileInfo, link)
	if err != nil {
		return
	}
	header.Name = destPath

	err = i.cpioWriter.WriteHeader(header)
	if err != nil {
		return
	}

	// Special files (unix sockets, directories, symlinks, ...) need to be handled differently
	// since a simple byte transfer of the file's content into the CPIO archive can't be achieved.
	if !fileInfo.Mode().IsRegular() {
		// For a symlink the reported size will be the size (in bytes) of the link's target.
		// Write this data into the archive.
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			_, err = i.cpioWriter.Write([]byte(link))
		}

		// For all other special files, they will be of size 0 and only contain the header in the archive.
		logger.Log.Debugf("Added special file %s", header.Name)
		return
	}

	bytesIO, err = io.Copy(i.cpioWriter, file)
	if err != nil {
		return
	}

	logger.Log.Debugf("New file %s caused %d bytes to be transfered to new archive", header.Name, bytesIO)

	return
}
