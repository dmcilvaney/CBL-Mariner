// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package hello

import (
	"path/filepath"

	"microsoft.com/pkggen/imagegen/diskutils"
	"microsoft.com/pkggen/internal/logger"
)

// World is a sample public (starts with a capital letter, must be commented) function.
func World() string {
	return "Hello, world!"
}

func TestInitramfs() {
	// Find initramfs in current directory
	initramfsPathList, err := filepath.Glob("initrd.img*")
	if err != nil || len(initramfsPathList) != 1 {
		logger.Log.Errorf("could not find single initramfs (%v): %v", initramfsPathList, err)
	}

	// Open it for editing
	i, err := diskutils.OpenInitramfs(initramfsPathList[0])
	if err != nil {
		logger.Log.Errorf("Error: %v", err)
		return
	}

	// Add a folder
	err = i.AddFileToInitramfs("testdir", "testdir/")
	if err != nil {
		logger.Log.Errorf("Error: %v", err)
		return
	}

	// Add a file
	err = i.AddFileToInitramfs("testdir/helloworld.txt", "testdir/helloworld.txt")
	if err != nil {
		logger.Log.Errorf("Error: %v", err)
		return
	}

	// Write it back
	err = i.Close()
	if err != nil {
		logger.Log.Errorf("Error: %v", err)
		return
	}
}
