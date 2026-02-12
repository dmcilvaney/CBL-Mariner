// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package rpmrepomanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/packagerepo/repoutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/rpm"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
)

// CreateRepo will create an RPM repository at repoDir
func CreateRepo(repoDir string) (err error) {
	const (
		repoDataSubDir = "repodata"
		repoLockSubDir = ".repodata"
	)

	logger.Log.Debugf("Creating RPM repository in (%s)", repoDir)

	repoDataPath := filepath.Join(repoDir, repoDataSubDir)
	repoDataLockPath := filepath.Join(repoDir, repoLockSubDir)

	// Remove the repodata (and the repolock) if exists
	err = os.RemoveAll(repoDataPath)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	err = os.RemoveAll(repoDataLockPath)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	createRepoCmd, err := repoutils.FindCreateRepoCommand()
	if err != nil {
		return fmt.Errorf("unable to create repo:\n%w", err)
	}

	// Create a new repodata
	_, stderr, err := shell.Execute(createRepoCmd, "--compatibility", repoDir)
	if err != nil {
		logger.Log.Warn(stderr)
	}

	return
}

// CreateOrUpdateRepo will create an RPM repository at repoDir or update
// it if the metadata files already exist.
func CreateOrUpdateRepo(repoDir string) (err error) {
	// Check if createrepo command is available
	createRepoCmd, err := repoutils.FindCreateRepoCommand()
	if err != nil {
		return fmt.Errorf("unable to create repo:\n%w", err)
	}

	// Create or update repodata
	_, stderr, err := shell.Execute(createRepoCmd, "--compatibility", "--update", repoDir)
	if err != nil {
		logger.Log.Warn(stderr)
	}

	return
}

// NormalizeRpmFilenames scans the given directory for RPM files whose on-disk filenames
// don't match their canonical names (e.g., due to hash prefixes added by a package server).
// Mismatched files are renamed to their canonical names. The alreadyNormalized map is used
// to skip files that have already been checked in previous calls.
func NormalizeRpmFilenames(repoDir string, alreadyNormalized map[string]bool) (err error) {
	rpmSearch := filepath.Join(repoDir, "*.rpm")
	rpmFiles, err := filepath.Glob(rpmSearch)
	if err != nil {
		return
	}

	for _, rpmFile := range rpmFiles {
		baseName := filepath.Base(rpmFile)
		if alreadyNormalized[baseName] {
			continue
		}

		canonicalName := rpm.NormalizeRPMFileName(baseName)
		if canonicalName == baseName {
			alreadyNormalized[baseName] = true
			continue
		}

		canonicalPath := filepath.Join(repoDir, canonicalName)
		logger.Log.Warnf("Normalizing hash-prefixed RPM filename: '%s' -> '%s'", baseName, canonicalName)
		err = os.Rename(rpmFile, canonicalPath)
		if err != nil {
			err = fmt.Errorf("failed to rename '%s' to '%s':\n%w", baseName, canonicalName, err)
			return
		}

		alreadyNormalized[canonicalName] = true
	}

	return
}

// ValidateRpmPaths checks for any rpm filenames in the cache that don't match the expected output according to 'rpm -qp ...'.  It
// will return an error with all the mismatched pairs if it finds any.
func ValidateRpmPaths(repoDir string) (err error) {
	rpmSearch := filepath.Join(repoDir, "*.rpm")
	rpmFiles, err := filepath.Glob(rpmSearch)
	if err != nil {
		return
	}

	// Create a string builder for validation errors
	validationErrors := []string{}

	for _, rpmFile := range rpmFiles {
		// rpmFile is the real RPM filename on disk.
		// use rpm cli to check the rpmFile's reported package name
		// print a warning if the filename does not match the package name
		var stdout, stderr string
		stdout, stderr, err = shell.Execute("rpm", "-qp", rpmFile)
		if err == nil {
			calculatedRpmName := strings.Split(stdout, "\n")
			calculatedRpmFilename := fmt.Sprintf("%s.rpm", calculatedRpmName[0])
			if calculatedRpmFilename != filepath.Base(rpmFile) {
				logger.Log.Warnf("!!!!! Detected mismatched filename !!!!!!")
				logger.Log.Warnf("---- filename   == '%s'", filepath.Base(rpmFile))
				logger.Log.Warnf("---- calculated == '%s'", calculatedRpmFilename)
				validationErrors = append(validationErrors, fmt.Sprintf("'%s' != '%s'", filepath.Base(rpmFile), calculatedRpmFilename))
			}
		} else {
			err = fmt.Errorf("failed to validate rpm file '%s': '%s'", rpmFile, stderr)
			return
		}
	}

	if len(validationErrors) > 0 {
		// Now build a single string for the validation errors
		errorBuilder := strings.Builder{}
		errorBuilder.WriteString("RPM filename validation errors: ")
		for _, validationError := range validationErrors {
			errorBuilder.WriteString(validationError)
			errorBuilder.WriteString(", ")
		}

		err = fmt.Errorf(errorBuilder.String())
	}

	return
}
