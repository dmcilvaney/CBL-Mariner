// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/directory"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/exe"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	newschedulertasks "github.com/microsoft/azurelinux/toolkit/tools/tasker/buildtasks"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("newsched", "Scheduler testing")

	specPaths  = app.Flag("specs", "Spec files to build").Required().ExistingFiles()
	specData   = app.Flag("spec-data", "Path to the spec data file.").Required().ExistingFile()
	fakePmcDir = app.Flag("fake-pmc-dir", "Path to 'PMC', which is actually just a directory of RPMs.").Required().ExistingDir()

	buildDirPath = app.Flag("build-dir", "Directory to store temporary files.").Required().String()
	repoRootDir  = app.Flag("repo-root", "Root directory of the repository.").Required().ExistingDir()
	workerTar    = app.Flag("worker-tar", "Full path to worker_chroot.tar.gz.").Required().ExistingFile()

	distTag = app.Flag("dist-tag", "The distribution tag.").Required().String()

	logFlags = exe.SetupLogFlags(app)
)

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(logFlags)

	task.InitializeLimiter(context.TODO(), 1)

	err := directory.EnsureDirExists(*buildDirPath)
	if err != nil {
		logger.Log.Fatalf("Failed to create build directory: %s", err)
	}

	buildConfig := buildconfig.BuildConfig{
		Arch:                  "x86_64",
		DistTag:               *distTag,
		RpmsDirsByDirtLevel:   make(map[int]string),
		SrpmsDirsByDirtLevel:  make(map[int]string),
		RpmsCacheDir:          filepath.Join(*buildDirPath, "RPMS-cache"),
		InputRepoDir:          *fakePmcDir,
		DoCheck:               false,
		MaxDirt:               3,
		AllowCacheForAnyLevel: true,
	}
	buildConfig.RpmsDirsByDirtLevel[0] = filepath.Join(*buildDirPath, "RPMS")
	buildConfig.SrpmsDirsByDirtLevel[0] = filepath.Join(*buildDirPath, "SRPMS")
	for i := 1; i <= buildConfig.MaxDirt; i++ {
		buildConfig.RpmsDirsByDirtLevel[i] = filepath.Join(*buildDirPath, "RPMS-dirty", fmt.Sprintf("%d", i))
		buildConfig.SrpmsDirsByDirtLevel[i] = filepath.Join(*buildDirPath, "SRPMS-dirty", fmt.Sprintf("%d", i))
	}
	buildconfig.CurrentBuildConfig = buildConfig

	// TODO: This is a hack to get the spec data into the specs package quickly
	toolkit_types.NewSpecDataDB(*specData)

	s := task.NewScheduler(true)

	goals := []*newschedulertasks.BuildSpecFileTask{}
	//for _, specPath := range *specPaths {
	spec := s.AddTask(
		nil,
		newschedulertasks.NewBuildSpecFileTask(
			(*specPaths)[0],
			0,
			buildconfig.CurrentBuildConfig,
		)).(*newschedulertasks.BuildSpecFileTask)
	goals = append(goals, spec)
	//}

	for _, goal := range goals {
		finalSpec := goal.Value()
		logger.Log.Warnf("Spec: %s DONE!", finalSpec.Path)
		for _, rpm := range finalSpec.ProvidedRpms {
			logger.Log.Warnf("  RPM: %s", rpm.Path)
		}
	}

	graphFilePath := filepath.Join(*buildDirPath, "graph.dot")
	graphFile, err := os.Create(graphFilePath)
	if err != nil {
		logger.Log.Fatalf("Failed to open graph file: %s", err)
	}
	defer graphFile.Close()
	s.WriteDOTGraph(graphFile)
	logger.Log.Infof("Wrote graph to %s", graphFilePath)
}
