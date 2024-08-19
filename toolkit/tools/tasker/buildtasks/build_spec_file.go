// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/directory"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/rpm"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"github.com/sirupsen/logrus"
)

type BuildSpecFileTask struct {
	task.DefaultValueTask[toolkit_types.SpecFile]
	// Input
	srpmFile *toolkit_types.SrpmFile
	// Output
	specFile *toolkit_types.SpecFile
}

func NewBuildSpecFileTask(path string, dirtLevel int, buildConfig buildconfig.BuildConfig) *BuildSpecFileTask {
	newSpecTask := &BuildSpecFileTask{
		specFile: toolkit_types.NewSpecFile(path, dirtLevel, buildConfig),
	}
	// newSpecTask.BasicTaskID = "SPEC_" + newSpecTask.specFile.Path
	// newSpecTask.BasicTaskName = fmt.Sprintf("SPEC: path=%s", newSpecTask.specFile.Path)
	// newSpecTask.DirtyLevel = dirtLevel
	newSpecTask.SetInfo(
		fmt.Sprintf("BUILDSPEC%d_%s", dirtLevel, path),
		fmt.Sprintf("BUILDSPEC: %s", filepath.Base(path)),
		dirtLevel,
	)

	//logger.Log.Infof("Creating new task: '%s'", newSpecTask.ID())
	return newSpecTask
}

func (s *BuildSpecFileTask) Execute() {
	s.TLog(logrus.InfoLevel, "Execute(): '%s'", s.ID())

	s.srpmFile = s.AddDependency(
		NewSrpmFileTask(s.specFile, false, s.DirtyLevel()),
	).(*SrpmFileTask).Value()

	// Enqueue build dependencies
	for _, dep := range s.srpmFile.BuildRequires {
		if !strings.HasPrefix(dep.Name, "rpmlib") {
			newDep := s.AddDependency(
				NewRpmCapibilityTask(dep, s.DirtyLevel()),
			).(*RpmCapibilityTask)
			if newDep == nil {
				s.TLog(logrus.FatalLevel, "Failed to create RPM Capability Task for: %s", dep)
			}
		}
	}
	s.TLog(logrus.InfoLevel, "%s waiting on:", s.ID())
	for _, dep := range s.ListDeps() {
		s.TLog(logrus.InfoLevel, "--  %s", dep)
	}

	s.WaitForDeps()

	// Build the package
	s.buildSpecFile(s.specFile, s.srpmFile.BuildRequires, buildconfig.CurrentBuildConfig)

	s.SetValue(*s.specFile)
	s.SetDone()
}

func (s *BuildSpecFileTask) buildSpecFile(specFile *toolkit_types.SpecFile, deps []*pkgjson.PackageVer, buildConfig buildconfig.BuildConfig) {
	workDir := s.GetWorkDir()
	defer os.RemoveAll(workDir)
	topDir := filepath.Join(workDir, "topdir")
	srpmDir := filepath.Join(topDir, "SRPMS")

	err := directory.EnsureDirExists(srpmDir)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to create SRPMS dir: %v", err)
	}
	tempSrpmPath := filepath.Join(srpmDir, filepath.Base(s.srpmFile.Path))
	err = file.Copy(s.srpmFile.Path, tempSrpmPath)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to copy SRPM file: %v", err)
	}

	macros := rpm.DefaultDistroDefines(buildConfig.DoCheck, buildConfig.DistTag)
	if s.DirtyLevel() > 0 {
		macros["dist"] = fmt.Sprintf("%s.dirty_%d", macros["dist"], s.DirtyLevel())
	}

	err = rpm.BuildRPMFromSRPM(tempSrpmPath, buildConfig.Arch, topDir, deps, macros, false, s.DirtyLevel()+1)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to build RPM from SRPM: %s", err)
	}

	// Get the built RPMs
	rpmList, err := moveBuiltRPMs(topDir, buildConfig.RpmsDirsByDirtLevel[s.DirtyLevel()])
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to move built RPMs: %s", err)
	}

	// Update the spec file with the real RPMs
	specFile.ProvidedRpms = nil
	for _, rpm := range rpmList {
		specFile.ProvidedRpms = append(specFile.ProvidedRpms, toolkit_types.NewRpmFileWithCapabilitiesFromRealFile(rpm))
	}
	sort.Slice(specFile.ProvidedRpms, func(i, j int) bool {
		return specFile.ProvidedRpms[i].Path < specFile.ProvidedRpms[j].Path
	})
	s.specFile = specFile
}

func moveBuiltRPMs(topDir, dstDir string) (builtRPMs []string, err error) {
	const (
		chrootRpmBuildDir = "RPMS"
		rpmExtension      = ".rpm"
	)

	rpmOutDir := filepath.Join(topDir, chrootRpmBuildDir)
	err = filepath.Walk(rpmOutDir, func(path string, info os.FileInfo, fileErr error) (err error) {
		if fileErr != nil {
			return fileErr
		}

		// Only copy regular files (not unix sockets, directories, links, ...)
		if !info.Mode().IsRegular() {
			return
		}

		if !strings.HasSuffix(path, rpmExtension) {
			return
		}

		// Get the relative path of the RPM, this will include the architecture directory it lives in.
		// Then join the relative path to the destination directory, this will ensure the RPM gets placed
		// in its correct architecture directory.
		relPath, err := filepath.Rel(rpmOutDir, path)
		if err != nil {
			return
		}

		dstFile := filepath.Join(dstDir, relPath)
		err = file.Move(path, dstFile)
		if err != nil {
			return
		}

		builtRPMs = append(builtRPMs, dstFile)
		return
	})

	return
}
