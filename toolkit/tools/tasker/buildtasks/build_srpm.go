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
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"github.com/sirupsen/logrus"
)

const (
	doIncludeDependencies = false
	doSkipDependencies    = true
)

type SrpmFileTask struct {
	task.DefaultValueTask[*toolkit_types.SrpmFile]
	srpmFile *toolkit_types.SrpmFile
}

func NewSrpmFileTask(sourceSpec *toolkit_types.SpecFile, dirtLevel int) *SrpmFileTask {
	newSrpmTask := &SrpmFileTask{
		srpmFile: toolkit_types.NewSrpmFile(sourceSpec),
	}
	newSrpmTask.SetInfo(
		fmt.Sprintf("BUILDSRPM%d_SRC_%s", dirtLevel, newSrpmTask.srpmFile.SourceSpec.Path),
		fmt.Sprintf("SRPM: %s", filepath.Base(newSrpmTask.srpmFile.SourceSpec.Path)),
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

	// Start with the .nosrc.rpm, add deps until we can build the real .src.rpm
	depList := make([]*pkgjson.PackageVer, 0)
	// Iterate until the dep lists converge
	changed := true
	i := 1
	for changed {
		// Get the .nosrc.rpm
		s.TLog(logrus.InfoLevel, "Iterating nosrc.rpm build to find deps %d", i)
		shouldDoDynamic := i > 1
		s.buildSrpm(s.srpmFile, depList, doSkipDependencies, shouldDoDynamic, buildconfig.CurrentBuildConfig)
		// Record the any possible new deps
		changed = false
		for _, dep := range s.srpmFile.BuildRequires {
			if !strings.HasPrefix(dep.Name, "rpmlib") {
				alreadyFound := sliceutils.Contains(depList, dep, sliceutils.PackageVerMatch)

				if !alreadyFound {
					changed = true
					depList = append(depList, dep)
					depTask := s.AddDependency(
						NewRpmCapibilityTask(dep, s.DirtyLevel()),
					)
					if depTask == nil {
						s.TLog(logrus.InfoLevel, "Would create cycle for SRPM build, incrementing dirt level")
						depTask = s.AddDependency(
							NewRpmCapibilityTask(dep, s.DirtyLevel()+1),
						)
					}
					if depTask == nil {
						s.TLog(logrus.FatalLevel, "Failed to add dependency for SRPM build: %s", dep)
					}
				}
			}
		}
		s.WaitForDeps()
		i++
	}

	s.TLog(logrus.InfoLevel, "%s waiting on:", s.ID())
	for _, dep := range s.ListDeps() {
		s.TLog(logrus.InfoLevel, "  %s", dep)
	}

	// //TODO: This is a stupid hack: List the rpm paths we want to include
	// rpmDepPaths := make([]string, 0)
	// for _, dep := range depTasks {
	// 	rpmDepPaths = append(rpmDepPaths, dep.Value().MappedPackage.Path)
	// }

	s.buildSrpm(s.srpmFile, depList, doIncludeDependencies, true, buildconfig.CurrentBuildConfig)

	s.SetValue(s.srpmFile)
	s.SetDone()
}

// Build the .src.rpm and update the path in the srpmFile
func (s *SrpmFileTask) buildSrpm(srpm *toolkit_types.SrpmFile, deps []*pkgjson.PackageVer, noDeps, doDynamic bool, buildConfig buildconfig.BuildConfig) {
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
	if noDeps {
		s.prepDummySources(topDir, srpm.SourceSpec)
		srpmPath, err = rpm.GenerateNoSRPMFromSPEC(tmpSpecPath, topDir, deps, macros, s.DirtyLevel()+1, len(deps) > 0)
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
	srpm.BuildRequires = make([]*pkgjson.PackageVer, 0)
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
