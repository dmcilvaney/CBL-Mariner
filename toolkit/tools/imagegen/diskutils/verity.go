// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Utility to encrypt disks and partitions

package diskutils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"microsoft.com/pkggen/imagegen/configuration"
	"microsoft.com/pkggen/internal/logger"
	"microsoft.com/pkggen/internal/miscutils"
	"microsoft.com/pkggen/internal/shell"
)

const (
	mappingVerityPrefix = "verity-"
)

//VerityDevice represents a device mapper linear device. The BackingDevice is the device used to create
// the linear device at MappedDevice with name MappedName
// - MappedName is the desired device mapper name
// - MappedDevice is the full path of the created device mapper device
// - BackingDevice is the underlying file/device which backs the partition
// - FecRoots is the number of error correcting roots, 0 to ommit error correction
// - TmpfsOverlays is a list of tmpfs overlays which should be created after the verity partition is mounted
type VerityDevice struct {
	MappedName           string
	MappedDevice         string
	BackingDevice        string
	FecRoots             int
	ValidateOnBoot       bool
	UseRootHashSignature bool
	ErrorBehaviour       string
	TmpfsOverlays        []string
}

func initramfsWorkAround(workingFolder, initramfsPath string, filesToAdd []string) (err error) {
	mountdir := filepath.Join(workingFolder, "initramfs_mnt")
	shell.Execute("mkdir", mountdir)

	gzipInitramfs := filepath.Join(workingFolder, filepath.Base(initramfsPath)+".gz")
	cpioInitramfs := filepath.Join(workingFolder, filepath.Base(initramfsPath))

	_, stderr, err := shell.Execute("mv", initramfsPath, gzipInitramfs)
	if err != nil {
		logger.Log.Error(stderr)
		return err
	}

	_, stderr, err = shell.Execute("gunzip", "-d", gzipInitramfs)
	if err != nil {
		logger.Log.Error(stderr)
		return err
	}

	_, stderr, err = shell.Execute("bash", "-c", "pushd "+mountdir+" && cpio -i < ../"+filepath.Base(cpioInitramfs)+" && popd")
	if err != nil {
		logger.Log.Error(stderr)
		return err
	}

	for _, file := range filesToAdd {
		baseName := filepath.Base(file)
		dst := filepath.Join(mountdir, baseName)
		_, stderr, err = shell.Execute("cp", file, dst)
	}

	_, stderr, err = shell.Execute("bash", "-c", "pushd "+mountdir+" && find . | cpio -H newc -o > ../"+filepath.Base(cpioInitramfs)+" ; popd")
	if err != nil {
		logger.Log.Error(stderr)
		return err
	}

	_, stderr, err = shell.Execute("gzip", cpioInitramfs)
	if err != nil {
		logger.Log.Error(stderr)
		return err
	}

	_, stderr, err = shell.Execute("mv", gzipInitramfs, initramfsPath)
	if err != nil {
		logger.Log.Error(stderr)
		return err
	}

	return

}

// AddRootVerityFilesToInitramfs adds files needed for a verity root to the initramfs
// - workingFolder is a temporary folder to extract the initramfs to
// - initramfsPath is the path to the initramfs
//func AddRootVerityFilesToInitramfs(destinationPath, initramfsDevice string, readOnlyDevice configuration.ReadOnlyVerityRoot) (err error) {
func (v *VerityDevice) AddRootVerityFilesToInitramfs(workingFolder, initramfsPath string) (err error) {
	var (
		verityWorkingDirectory = filepath.Join(workingFolder, v.MappedName)
	)

	logger.Log.Warnf("Starting verity...")
	// Measure the disk and generate the hash and fec files
	err = v.createVerityDisk(verityWorkingDirectory)
	if err != nil {
		return fmt.Errorf("failed while generating a verity disk: %w", err)
	}

	//runtime.Breakpoint()
	// Now place them in the initramfs
	// initramfs, err := OpenInitramfs(initramfsPath)
	// defer initramfs.Close()
	// if err != nil {
	// 	return fmt.Errorf("failed to open the initramfs: %w", err)
	// }

	logger.Log.Warnf("Starting getting verity output files")

	verityFiles, err := ioutil.ReadDir(verityWorkingDirectory)
	if err != nil {
		return
	}

	logger.Log.Warnf("Starting verity output files")

	// WORKAROUND FIX ME!
	verityFileList := []string{}
	for _, f := range verityFiles {
		verityFileList = append(verityFileList, filepath.Join(verityWorkingDirectory, f.Name()))
	}
	initramfsWorkAround(verityWorkingDirectory, initramfsPath, verityFileList)

	// for _, file := range verityFiles {
	// 	filePath := filepath.Join(verityWorkingDirectory, file.Name())
	// 	// Place each file in the root of the initramfs
	// 	err = initramfs.AddFileToInitramfs(filePath, file.Name())
	// 	if err != nil {
	// 		return fmt.Errorf("failed adding %s to initramfs: %w", filePath, err)
	// 	}
	// }

	return
}

func (v *VerityDevice) createVerityDisk(verityDirectory string) (err error) {
	const (
		hexChars   = "0123456789abcdef"
		saltLength = 64
		hashAlg    = "sha256"
	)
	var (
		verityFecArgs    []string
		verityArgs       []string
		verityVerifyArgs []string
		fileBase         = filepath.Join(verityDirectory, v.MappedName)
		logFilePath      = fmt.Sprintf("%s.log", fileBase)
		rootHashPath     = fmt.Sprintf("%s.roothash", fileBase)
		hashtreePath     = fmt.Sprintf("%s.hashtree", fileBase)
	)

	err = os.MkdirAll(verityDirectory, os.ModePerm)
	if err != nil {
		return
	}

	logFile, err := os.Create(logFilePath)
	if err != nil {
		return
	}
	defer logFile.Close()

	// logFile.WriteString("Hello world")
	// if err == nil {
	// 	return
	// }

	rootHashFile, err := os.Create(rootHashPath)
	if err != nil {
		return
	}
	defer rootHashFile.Close()

	salt, err := miscutils.RandomString(64, hexChars)
	if err != nil {
		return
	}

	if v.FecRoots > 0 {
		verityFecArgs = []string{
			fmt.Sprintf("--fec-device=%s.fec", fileBase),
			fmt.Sprintf("--fec-roots=%d", v.FecRoots),
		}
	}

	verityArgs = []string{
		"--salt",
		salt,
		"--hash",
		hashAlg,
		"--verbose",
		"--debug",
		"format",
		v.MappedDevice,
		hashtreePath,
	}

	logger.Log.Info("Generating dm verity read-only partition, this may take several minutes")
	verityOutput, stderr, err := shell.Execute("veritysetup", append(verityFecArgs, verityArgs...)...)
	if err != nil {
		err = fmt.Errorf("Unable to create verity disk '%s': %w", stderr, err)
		return
	}
	_, err = logFile.WriteString(verityOutput)
	if err != nil {
		return
	}

	// Searches for a line like: "Root hash:      1234567890abcdefg..."
	rootHashLine, stderr, err := shell.ExecuteWithStdin(verityOutput, "grep", "^Root hash:")
	if err != nil {
		err = fmt.Errorf("Unable to find root hash '%s': %w", stderr, err)
		return
	}
	// Print the last entry in the line
	// ORS (output record separator) inserts a newline normally, explicitly skip it
	rootHash, stderr, err := shell.ExecuteWithStdin(rootHashLine, "awk", `BEGIN {ORS=""}; END{ print $NF }`)
	if err != nil {
		err = fmt.Errorf("Unable extract root hash '%s': %w", stderr, err)
		return
	}

	logger.Log.Infof("Verity partition completed, root hash: '%s'", rootHash)
	_, err = rootHashFile.WriteString(rootHash)
	if err != nil {
		return
	}

	//Verify the disk was created correctly:
	verityVerifyArgs = []string{
		"--verbose",
		"--debug",
		"verify",
		v.MappedDevice,
		hashtreePath,
		rootHash,
	}

	logger.Log.Info("Verifying the verity partition")
	verityOutput, stderr, err = shell.Execute("veritysetup", verityVerifyArgs...)
	if err != nil {
		logger.Log.Error(verityOutput)
		time.Sleep(time.Second)
		err = fmt.Errorf("Unable to validate new verity disk '%s': %w", stderr, err)
	}

	return
}

// PrepReadOnlyDevice sets up a device mapper linear map to the loopback device.
// This map will have the correct name of the final verity disk, and can be
// switched to read-only when the final image is ready for measurement.
// - partDevPath is the path of the root partition (likely a loopback device)
// - partition is the configuration
// - encrypt is the root encryption settings
func PrepReadOnlyDevice(partDevPath string, partition configuration.Partition, readOnlyConfig configuration.ReadOnlyVerityRoot) (readOnlyDevice VerityDevice, err error) {
	const (
		linearTable = `0 %d linear %s 0`
	)

	if !readOnlyConfig.Enable {
		err = fmt.Errorf("Verity is not enabled, can't update partition '%s'", partition.ID)
		return
	}
	finalDeviceName := fmt.Sprintf("%s%s", mappingVerityPrefix, readOnlyConfig.Name)
	readOnlyDevice.BackingDevice = partDevPath
	readOnlyDevice.MappedName = finalDeviceName
	readOnlyDevice.MappedDevice = filepath.Join("/dev/mapper/", finalDeviceName)
	readOnlyDevice.ErrorBehaviour = readOnlyConfig.VerityErrorBehavior.String()
	readOnlyDevice.ValidateOnBoot = readOnlyConfig.ValidateOnBoot
	if readOnlyConfig.ErrorCorrectionEnable {
		readOnlyDevice.FecRoots = readOnlyConfig.ErrorCorrectionEncodingRoots
	} else {
		readOnlyDevice.FecRoots = 0
	}
	readOnlyDevice.TmpfsOverlays = readOnlyConfig.TmpfsOverlays
	readOnlyDevice.UseRootHashSignature = readOnlyConfig.RootHashSignatureEnable

	// linear mappings need to know the size of the disk in blocks ahead of time
	deviceSizeStr, stderr, err := shell.Execute("blockdev", "--getsz", readOnlyDevice.BackingDevice)
	if err != nil {
		err = fmt.Errorf("unable to get loopback device size %s. Error: %s", partDevPath, stderr)
		return
	}
	deviceSizeInt, err := strconv.ParseUint(strings.TrimSpace(deviceSizeStr), 10, 64)
	if err != nil {
		err = fmt.Errorf("Unable to convert disk size '%s' to integer: %w", deviceSizeStr, err)
		return
	}

	populatedTable := fmt.Sprintf(linearTable, deviceSizeInt, readOnlyDevice.BackingDevice)
	dmsetupArgs := []string{
		"create",
		readOnlyDevice.MappedName,
		"--table",
		populatedTable,
	}
	_, stderr, err = shell.Execute("dmsetup", dmsetupArgs...)
	if err != nil {
		err = fmt.Errorf("Unable to create a device mapper device '%s': %w", stderr, err)
		return
	}

	logger.Log.Infof("Remapped partition %s for read-only prep to %s", partition.ID, readOnlyDevice.MappedDevice)

	return
}

// CleanupVerityDevice removes the device mapper linear mapping, but leaves the backing device unchanged
func (v *VerityDevice) CleanupVerityDevice() {
	stdout, stderr, err := shell.Execute("dmsetup", "remove", v.MappedName)
	logger.Log.Infof(stdout)
	if err != nil {
		err = fmt.Errorf("Unable to clean up device mapper device '%s' '%s': %w", stdout, stderr, err)
		return
	}
}

// SwitchDeviceToReadOnly switches the root device linear map to read only
// Will also re-mount the moint point to respect this.
// - mountPointOrDevice is either the location of the mount, or the device which was mounted (mount command will take either)
// - mountArgs are any special mount options used which should continue to be used
func (v *VerityDevice) SwitchDeviceToReadOnly(mountPointOrDevice, mountArgs string) (err error) {
	const (
		remountOptions = "remount,ro"
	)

	// Suspending the mapped device will force a sync
	_, stderr, err := shell.Execute("dmsetup", "suspend", v.MappedName)
	if err != nil {
		return fmt.Errorf("failed to suspend device '%s' : '%v': %w", v.MappedDevice, stderr, err)
	}

	// Need to get the table data to "recreate" the device with read-only set
	table, stderr, err := shell.Execute("dmsetup", "table", v.MappedName)
	if err != nil {
		return fmt.Errorf("failed to get table for device '%s' : '%v': %w", v.MappedDevice, stderr, err)
	}

	// Switch the linear map to read-only
	dmsetupArgs := []string{
		"reload",
		"--readonly",
		v.MappedName,
		"--table",
		table,
	}
	_, stderr, err = shell.Execute("dmsetup", dmsetupArgs...)
	if err != nil {
		return fmt.Errorf("failed to reload device '%s' in read-only mode  : '%v': %w", v.MappedDevice, stderr, err)
	}

	// Re-enable the device
	_, stderr, err = shell.Execute("dmsetup", "resume", v.MappedName)
	if err != nil {
		return fmt.Errorf("failed to resume device '%s' : '%v': %w", v.MappedDevice, stderr, err)
	}

	// Mounts don't respect the read-only nature of the underlying device, force a remount
	_, stderr, err = shell.Execute("mount", "-o", mountArgs+remountOptions, mountPointOrDevice)
	if err != nil {
		return fmt.Errorf("failed to remount '%s': '%s': %w", mountPointOrDevice, stderr, err)
	}
	return
}

// IsReadOnlyDevice checks if a given device is a dm-verity read-only device
// - devicePath is the device to check
func IsReadOnlyDevice(devicePath string) (result bool) {
	verityPrefix := filepath.Join(mappingFilePath, mappingVerityPrefix)
	if strings.HasPrefix(devicePath, verityPrefix) {
		return true
	}
	return
}
