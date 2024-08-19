// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"fmt"
	"os"
	"path/filepath"
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

type SrpmFileTask struct {
	task.DefaultValueTask[*toolkit_types.SrpmFile]
	srpmFile *toolkit_types.SrpmFile
}

func NewSrpmFileTask(sourceSpec *toolkit_types.SpecFile, noDeps bool, dirtLevel int) *SrpmFileTask {
	newSrpmTask := &SrpmFileTask{
		srpmFile: toolkit_types.NewSrpmFile(sourceSpec, noDeps),
	}
	var name string
	if noDeps {
		name = fmt.Sprintf("SRPM(nodeps): %s", filepath.Base(newSrpmTask.srpmFile.SourceSpec.Path))
	} else {
		name = fmt.Sprintf("SRPM: %s", filepath.Base(newSrpmTask.srpmFile.SourceSpec.Path))
	}
	newSrpmTask.SetInfo(
		fmt.Sprintf("BUILDSRPM_NODEPS%d_%t_SRC_%s", dirtLevel, newSrpmTask.srpmFile.NoDeps, newSrpmTask.srpmFile.SourceSpec.Path),
		name,
		dirtLevel,
	)

	//logger.Log.Infof("Creating new task: '%s'", newSrpmTask.ID())
	return newSrpmTask
}

// Steps to build a spec:
// 1. Get the .nosrc.rpm
// 2. Gather build dependencies from it, and enqueue them
// 3. Build the .src.rpm
func (s *SrpmFileTask) Execute() {
	s.TLog(logrus.InfoLevel, "Execute(): '%s'", s.ID())
	if s.srpmFile.NoDeps {
		// Don't need any dependencies here
		s.buildSrpm(s.srpmFile, nil, buildconfig.CurrentBuildConfig)
	} else {
		// We need the .nosrc.rpm to get the build dependencies, queue it up.
		noSrcRpm := s.AddDependency(
			NewSrpmFileTask(s.srpmFile.SourceSpec, true, s.DirtyLevel()),
		).(*SrpmFileTask).Value()
		// Enqueue build dependencies
		filteredDeps := make([]*pkgjson.PackageVer, 0)
		for _, dep := range noSrcRpm.BuildRequires {
			//TODO: Ignore rpmlib* dependencies for now
			if !strings.HasPrefix(dep.Name, "rpmlib") {
				s.AddDependency(
					NewRpmCapibilityTask(dep, s.DirtyLevel()),
				)
				filteredDeps = append(filteredDeps, dep)
			}
		}
		s.TLog(logrus.InfoLevel, "%s waiting on:", s.ID())
		for _, dep := range s.ListDeps() {
			s.TLog(logrus.InfoLevel, "  %s", dep)
		}

		s.WaitForDeps()

		// //TODO: This is a stupid hack: List the rpm paths we want to include
		// rpmDepPaths := make([]string, 0)
		// for _, dep := range depTasks {
		// 	rpmDepPaths = append(rpmDepPaths, dep.Value().MappedPackage.Path)
		// }

		s.buildSrpm(s.srpmFile, filteredDeps, buildconfig.CurrentBuildConfig)
	}

	s.SetValue(s.srpmFile)
	s.SetDone()
}

// Build the .src.rpm and update the path in the srpmFile
func (s *SrpmFileTask) buildSrpm(srpm *toolkit_types.SrpmFile, deps []*pkgjson.PackageVer, buildConfig buildconfig.BuildConfig) {
	l := task.AcquireTaskLimiter(1)
	defer l.Release()

	var err error

	workDir := s.GetWorkDir()
	defer os.RemoveAll(workDir)
	topDir := filepath.Join(workDir, "topdir")
	specsDir := filepath.Join(topDir, "SOURCES")

	err = directory.EnsureDirExists(specsDir)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to create sources dir: %v", err)
	}
	tmpSpecPath := filepath.Join(specsDir, filepath.Base(srpm.SourceSpec.Path))
	err = file.Copy(srpm.SourceSpec.Path, tmpSpecPath)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to copy spec file: %v", err)
	}

	var srpmPath string
	macros := rpm.DefaultDistroDefines(buildConfig.DoCheck, buildConfig.DistTag)
	if s.DirtyLevel() > 0 {
		macros["dist"] = fmt.Sprintf("%s.dirty_%d", macros["dist"], s.DirtyLevel())
	}
	if srpm.NoDeps {
		s.prepDummySources(topDir, srpm.SourceSpec)
		srpmPath, err = rpm.GenerateNoSRPMFromSPEC(tmpSpecPath, topDir, nil, macros, 0)
	} else {
		s.prepRealSources(topDir, srpm.SourceSpec)
		srpmPath, err = rpm.GenerateSRPMFromSPEC(tmpSpecPath, topDir, deps, macros, s.DirtyLevel()+1)
	}
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to generate SRPM: %v", err)
	}

	srpm.Path = filepath.Join(buildConfig.SrpmsDirsByDirtLevel[s.DirtyLevel()], filepath.Base(srpmPath))

	// Set BRs
	allBRs, err := rpm.QueryRPMRequires2(srpmPath)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to query SRPM requires: %v", err)
	}
	for _, br := range allBRs {
		if !strings.HasPrefix(br.Name, "rpmlib") {
			srpm.BuildRequires = append(srpm.BuildRequires, br)
		}
	}

	err = file.Move(srpmPath, srpm.Path)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to move SRPM: %v", err)
	}
}

// Add empty files for each source
func (s *SrpmFileTask) prepDummySources(topDir string, spec *toolkit_types.SpecFile) {
	sourcesDir := filepath.Join(topDir, "SOURCES")
	err := directory.EnsureDirExists(sourcesDir)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to create sources dir: %v", err)
	}

	for _, source := range spec.Sources {
		err = file.Create(filepath.Join(sourcesDir, source), os.ModePerm)
		if err != nil {
			s.TLog(logrus.FatalLevel, "Failed to create source file: %v", err)
		}
	}
}

func (s *SrpmFileTask) prepRealSources(topDir string, spec *toolkit_types.SpecFile) {
	//TODO implement this, we assume all files are next to the .spec for now.
	sourcesDir := filepath.Join(topDir, "SOURCES")
	err := directory.EnsureDirExists(sourcesDir)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to create sources dir: %v", err)
	}

	for _, source := range spec.Sources {
		srcPath := filepath.Join(filepath.Dir(spec.Path), source)
		err := file.Copy(srcPath, filepath.Join(sourcesDir, source))
		if err != nil {
			s.TLog(logrus.FatalLevel, "Failed to copy source file: %v", err)
		}
	}
}
