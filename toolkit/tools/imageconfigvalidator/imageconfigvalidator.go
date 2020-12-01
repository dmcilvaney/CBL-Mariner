// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// An image configuration validator

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
	"microsoft.com/pkggen/imagegen/configuration"
	"microsoft.com/pkggen/imagegen/installutils"
	"microsoft.com/pkggen/internal/exe"
	"microsoft.com/pkggen/internal/logger"
)

var (
	app = kingpin.New("imageconfigvalidator", "A tool for validating image configuration files")

	logFile  = exe.LogFileFlag(app)
	logLevel = exe.LogLevelFlag(app)

	input       = exe.InputStringFlag(app, "Path to the image config file.")
	baseDirPath = exe.InputDirFlag(app, "Base directory for relative file paths from the config. Defaults to config's directory.")
)

func main() {
	const returnCodeOnError = 1

	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(*logFile, *logLevel)

	inPath, err := filepath.Abs(*input)
	logger.PanicOnError(err, "Error when calculating input path")
	baseDir, err := filepath.Abs(*baseDirPath)
	logger.PanicOnError(err, "Error when calculating input directory")

	logger.Log.Infof("Reading configuration file (%s)", inPath)
	config, err := configuration.LoadWithAbsolutePaths(inPath, baseDir)
	if err != nil {
		logger.Log.Fatalf("Failed while loading image configuration '%s': %s", inPath, err)
	}

	// Basic validation will occur during load, but we can add additional checking here.
	err = ValidateConfiguration(config)
	if err != nil {
		// Log an error here as opposed to panicing to keep the output simple
		// and only contain the error with the config file.
		logger.Log.Fatalf("Invalid configuration '%s': %s", inPath, err)
	}

	return
}

// ValidateConfiguration will run sanity checks on a configuration structure
func ValidateConfiguration(config configuration.Config) (err error) {
	err = config.IsValid()
	if err != nil {
		return
	}
	err = validatePackages(config)
	return
}

func validatePackages(config configuration.Config) (err error) {
	const (
		validateError = "failed to validate package lists in config"
		verityPkgName = "verity-readonly-root"
	)
	for _, systemConfig := range config.SystemConfigs {
		packageList, err := installutils.PackageNamesFromSingleSystemConfig(systemConfig)
		if err != nil {
			return fmt.Errorf("%s: %w", validateError, err)
		}
		foundVerityInitramfsPackage := false
		for _, pkg := range packageList {
			if pkg == "kernel" {
				return fmt.Errorf("%s: kernel should not be included in a package list, add via config file's [KernelOptions] entry", validateError)
			}
			if pkg == verityPkgName {
				foundVerityInitramfsPackage = true
			}
		}
		if systemConfig.ReadOnlyVerityRoot.Enable && !foundVerityInitramfsPackage {
			return fmt.Errorf("%s: verity read only rootfs selected, but %s package is not included in the package lists", validateError, verityPkgName)
		}
	}
	return
}
