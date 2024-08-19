// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package main

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/debugutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/exe"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/rpm"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/simpletoolchroot"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("newsched", "Scheduler testing")

	specs = app.Flag("specs", "Spec files to build").Required().ExistingFiles()

	buildDirPath = app.Flag("build-dir", "Directory to store temporary files.").Required().ExistingDir()
	repoRootDir  = app.Flag("repo-root", "Root directory of the repository.").Required().ExistingDir()
	workerTar    = app.Flag("worker-tar", "Full path to worker_chroot.tar.gz.").Required().ExistingFile()

	distTag = app.Flag("dist-tag", "The distribution tag.").Required().String()

	logFlags = exe.SetupLogFlags(app)
)

// Queue task interface. This interface allows for the creation of a task which may be queued for execution. A task
// may require other tasks to be completed before it can be executed.
type Task interface {
	// Execute the task.
	Execute() error
	// Get the name of the task.
	Name() string
	// Get the tasks that must be completed before this task can be executed.
	Dependencies() []Task
}

type Spec struct {
	path                 string
	providesInitial      []*pkgjson.PackageVer
	requiresInitial      []*pkgjson.PackageVer
	buildRequiresInitial []*pkgjson.PackageVer
	buildRequiresFinal   []*pkgjson.PackageVer
}

type newSched struct {
	simpleChroot simpletoolchroot.SimpleToolChroot
	specs        []*Spec
	workDir      string
	defines      map[string]string
}

func (s *newSched) parseSpecList(paths []string) ([]*Spec, error) {
	for _, path := range paths {
		spec, err := s.parseSpec(path)
		if err != nil {
			return nil, err
		}
		s.specs = append(s.specs, spec)
	}
	return s.specs, nil
}

func (s *newSched) parseSpec(specPath string) (*Spec, error) {
	specName := path.Base(specPath)
	specWorkDir, err := os.MkdirTemp(s.workDir, specName)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(specWorkDir)

	newSpec := &Spec{
		path: specPath,
	}

	sourceDir := path.Dir(specPath)
	arch, _ := rpm.GetRpmArch(runtime.GOARCH)
	emptyQueryFormat := ""

	queryResults, err := rpm.QuerySPEC(specPath, sourceDir, emptyQueryFormat, arch, s.defines, rpm.BuildRequiresArgument)
	if err != nil {
		return nil, fmt.Errorf("failed to query spec:\n%w", err)
	}
	for _, result := range queryResults {
		ver, err := pkgjson.PackageStringToPackageVer(result)
		if err != nil {
			return nil, fmt.Errorf("failed to convert package string to package version:\n%w", err)
		}
		newSpec.buildRequiresInitial = append(newSpec.buildRequiresInitial, ver)
	}

	// // Parse the srpm to get the provides and requires
	// newSpec.providesInitial, err = rpm.QueryRPMProvides2(outPath)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to query provides:\n%w", err)
	// }

	// newSpec.requiresInitial, err = rpm.QueryRPMRequires2(outPath)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to query requires:\n%w", err)
	// }

	return newSpec, nil
}

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(logFlags)

	//mountDir := path.Join(*buildDirPath, "mnt")

	newSched := newSched{
		//simpleChroot: simpletoolchroot.SimpleToolChroot{},
		defines: rpm.DefaultDistroDefines(false, *distTag),
	}

	// if err := newSched.simpleChroot.InitializeChroot(*buildDirPath, "newsched", *workerTar, mountDir); err != nil {
	// 	logger.Log.Fatalf("Failed to initialize chroot: %v", err)
	// }
	// defer newSched.simpleChroot.CleanUp()

	var err error
	newSched.workDir, err = os.MkdirTemp(*buildDirPath, "working")
	if err != nil {
		logger.Log.Fatalf("Failed to create working directory: %v", err)
	}
	defer os.RemoveAll(newSched.workDir)

	specs, err := newSched.parseSpecList(*specs)
	if err != nil {
		logger.Log.Fatalf("Failed to parse spec files: %v", err)
	}

	for _, spec := range specs {
		logger.Log.Infof("Spec: %s", spec.path)
		for _, prov := range spec.providesInitial {
			logger.Log.Infof("Provides: %s", prov)
		}
		for _, req := range spec.requiresInitial {
			logger.Log.Infof("Requires: %s", req)
		}
	}

	debugutils.WaitForUser("test")
}
