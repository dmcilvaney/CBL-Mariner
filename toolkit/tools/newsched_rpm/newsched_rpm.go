// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/directory"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/exe"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	newschedulertasks "github.com/microsoft/azurelinux/toolkit/tools/tasker/buildtasks"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("newsched", "Scheduler testing")

	specPaths  = app.Flag("specs", "Spec files to build").ExistingFiles()
	specData   = app.Flag("spec-data", "Path to the spec data file.").Required().ExistingFile()
	fakePmcDir = app.Flag("fake-pmc-dir", "Path to 'PMC', which is actually just a directory of RPMs.").Required().ExistingDir()

	buildDirPath = app.Flag("build-dir", "Directory to store temporary files.").Required().String()
	repoRootDir  = app.Flag("repo-root", "Root directory of the repository.").Required().ExistingDir()
	workerTar    = app.Flag("worker-tar", "Full path to worker_chroot.tar.gz.").Required().ExistingFile()

	sourceUrl = app.Flag("source-url", "The source URL.").Required().String()

	distTag = app.Flag("dist-tag", "The distribution tag.").Required().String()

	logFlags = exe.SetupLogFlags(app)
)

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(logFlags)

	task.InitializeLimiter(context.TODO(), 50)

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
		MaxDirt:               2,
		AllowCacheForAnyLevel: true,
		SourceUrl:             *sourceUrl,
		TempDir:               filepath.Join(os.TempDir(), "azl-toolkit"),
		AddToolchainPackages:  false,
	}
	buildConfig.RpmsDirsByDirtLevel[0] = filepath.Join(*buildDirPath, "RPMS")
	buildConfig.SrpmsDirsByDirtLevel[0] = filepath.Join(*buildDirPath, "SRPMS")
	for i := 1; i <= buildConfig.MaxDirt; i++ {
		buildConfig.RpmsDirsByDirtLevel[i] = filepath.Join(*buildDirPath, "RPMS-dirty", fmt.Sprintf("%d", i))
		buildConfig.SrpmsDirsByDirtLevel[i] = filepath.Join(*buildDirPath, "SRPMS-dirty", fmt.Sprintf("%d", i))
	}
	buildconfig.CurrentBuildConfig = buildConfig

	err = directory.EnsureDirExists(buildconfig.CurrentBuildConfig.TempDir)
	if err != nil {
		logger.Log.Fatalf("Failed to create temp directory: %s", err)
	}

	// TODO: This is a hack to get the spec data into the specs package quickly
	toolkit_types.NewSpecDataDB(*specData)

	s := task.NewScheduler(false)
	s.StartProgressPrinter()

	cancel := configureGraphDebug(s)
	defer cancel()

	goals := []task.Tasker{}
	// spec := s.AddTask(
	// 	nil,
	// 	newschedulertasks.NewBuildSpecFileTask(
	// 		(*specPaths)[0],
	// 		0,
	// 		buildconfig.CurrentBuildConfig,
	// 	), task.NoSelfCycle).(*newschedulertasks.BuildSpecFileTask)
	// goals = append(goals, spec)
	cap := s.AddTask(
		nil,
		newschedulertasks.NewRpmCapibilityTask(
			&pkgjson.PackageVer{
				Name: "bash",
			},
			0,
		), task.NoSelfCycle)
	goals = append(goals, cap)

	for _, goal := range goals {
		// finalSpec := goal.Value()
		// logger.Log.Warnf("Spec: %s DONE!", finalSpec.Path)
		// for _, rpm := range finalSpec.ProvidedRpms {
		// 	logger.Log.Warnf("  RPM: %s", rpm.Path)
		// }
		finalCap := goal.(*newschedulertasks.RpmCapibilityTask).Value()
		logger.Log.Warnf("Cap: %s DONE!", finalCap.MappedPackage.Path)
	}
	logrus.Exit(0)
}

func configureGraphDebug(s *task.Scheduler) (cancel context.CancelFunc) {
	graphFileFullPath := filepath.Join(*buildDirPath, "graph_full.dot")
	graphFileCleanPath := filepath.Join(*buildDirPath, "graph.dot")

	// Dump the graph to a file on any exit
	logrus.RegisterExitHandler(func() {
		writeGraphs(s, graphFileFullPath, graphFileCleanPath)
		logger.Log.Infof("Wrote graph to %s", graphFileFullPath)
		logger.Log.Infof("Wrote clean graph to %s", graphFileCleanPath)
	})

	// Periodically dump the graph to a file
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// Do a write immediately, incrementing the deltay time by 1 second up to a max of
		// 5 minutes
		writeGraphs(s, graphFileFullPath, graphFileCleanPath)
		delayTime := 10 * time.Second
		for {
			writeGraphs(s, graphFileFullPath, graphFileCleanPath)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delayTime):
				if delayTime < 5*time.Minute {
					delayTime += 1 * time.Second
				}
				continue
			}
		}
	}()
	return cancel
}

func writeGraphs(s *task.Scheduler, graphFileFullPath, graphFileCleanPath string) {
	graphFileFull, err := os.Create(graphFileFullPath)
	if err != nil {
		logger.Log.Fatalf("Failed to open graph file: %s", err)
	}
	defer graphFileFull.Close()

	graphFileClean, err := os.Create(graphFileCleanPath)
	if err != nil {
		logger.Log.Fatalf("Failed to open graph file: %s", err)
	}
	defer graphFileClean.Close()

	s.WriteDOTGraph(graphFileFull, graphFileClean)
}
