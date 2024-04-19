// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// A tool for generating snapshots of built RPMs from local specs.

package main

import (
	"os"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/exe"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	packagelist "github.com/microsoft/azurelinux/toolkit/tools/internal/packlist"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/codesearch"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("rpmsspnapshot", "A tool to generate a snapshot of all RPMs expected to be built from given specs folder.")

	srpmDir     = app.Flag("srpm-dir", "The output directory for source RPM packages").Required().String()
	searchRegex = app.Flag("search-regex", "The regex to search for in the spec files.").Required().String()

	buildDirPath = app.Flag("build-dir", "Directory to store temporary files.").Required().String()
	distTag      = app.Flag("dist-tag", "The distribution tag.").Required().String()
	workerTar    = app.Flag("worker-tar", "Full path to worker_chroot.tar.gz.").Required().ExistingFile()
	srpmListFile = app.Flag("spec-list", "Path to a list of SPECs to parse. If empty will parse all SPECs.").ExistingFile()
	disableTmpfs = app.Flag("disable-tmpfs", "Don't use a tmpfs for the chroot environment to save on memory.").Default("false").Bool()

	logFlags         = exe.SetupLogFlags(app)
	searchResultFile = app.Flag("search-result-file", "The file to store the search result.").Default("").String()
)

func main() {
	var (
		err    error
		writer *os.File = nil
	)

	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(logFlags)

	// Prompt the user for a search regex if not provided
	if *searchRegex == "" {
		*searchRegex = exe.PromptUserForString("Enter the regex to search for in the spec files:")
	}

	if *searchResultFile != "" {
		logger.Log.Infof("Writing search result to file: %s", *searchResultFile)
		writer, err = os.Create(*searchResultFile)
		if err != nil {
			logger.Log.Fatalf("Failed to create search result file. Error: %v", err)
		}
		defer writer.Close()
	}

	packageListSet, err := packagelist.ParsePackageListFile(*srpmListFile)
	if err != nil {
		logger.Log.Fatalf("Failed to parse package list file. Error: %v", err)
	}

	logger.Log.Info("Preparing search environment...")
	codeSearch, err := codesearch.New(*buildDirPath, *workerTar, *srpmDir, writer, !*disableTmpfs)
	if err != nil {
		logger.Log.Fatalf("Failed to initialize RPM snapshot generator. Error: %v", err)
	}
	defer func() {
		cleanupErr := codeSearch.CleanUp()
		if cleanupErr != nil {
			logger.Log.Fatalf("Failed to cleanup snapshot generator. Error: %s", cleanupErr)
		}
	}()

	logger.Log.Infof("Searching SRPMS for regex '%s'...", *searchRegex)
	err = codeSearch.SearchCode(*searchRegex, *distTag, packageListSet)
	if err != nil {
		logger.Log.Fatalf("Failed to generate snapshot. Error: %v", err)
	}

	logger.Log.Infof("Searched using 'grep -rinP \"%s\"' ./path/to/BUILD/", *searchRegex)
	codeSearch.PrintResults()
}
