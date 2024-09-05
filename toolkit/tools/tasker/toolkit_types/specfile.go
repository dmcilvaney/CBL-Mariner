// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package toolkit_types

import (
	"path/filepath"
	"runtime"
	"sort"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/rpm"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
)

type SpecFile struct {
	Path              string
	ProvidedRpms      []*RpmFile
	PredictedProvides []*pkgjson.PackageVer
	Sources           []*SourceFile
}

func NewSpecFile(path string, dirtLevel int, buildConfig buildconfig.BuildConfig) *SpecFile {
	rpmsDir := buildConfig.RpmsDirsByDirtLevel[dirtLevel]
	distTag := buildConfig.DistTag
	doCheck := buildConfig.DoCheck

	arch, _ := rpm.GetRpmArch(runtime.GOARCH)
	builtRpms, err := rpm.QuerySPECForBuiltRPMsWithArchPath(path, filepath.Dir(path), arch, rpm.DefaultDistroDefines(doCheck, distTag))
	if err != nil {
		logger.Log.Fatalf("Failed to query spec file %s: %s", path, err)
	}
	sort.Strings(builtRpms)

	predictedProvides, err := rpm.QuerySPECForProvides(path, filepath.Dir(path), arch, rpm.DefaultDistroDefines(doCheck, distTag))
	if err != nil {
		logger.Log.Fatalf("Failed to query spec file %s: %s", path, err)
	}
	sort.Strings(predictedProvides)

	sources, patches, err := rpm.QuerySPECForSources(path, filepath.Dir(path), arch, rpm.DefaultDistroDefines(doCheck, distTag))
	if err != nil {
		logger.Log.Fatalf("Failed to query spec file %s: %s", path, err)
	}
	sort.Strings(sources)
	sort.Strings(patches)

	newSpec := &SpecFile{
		Path:         path,
		ProvidedRpms: nil,
		Sources:      []*SourceFile{},
	}
	for _, source := range sources {
		newSpec.Sources = append(newSpec.Sources, NewSourceFile(source, SourceFileTypeSource))
	}
	for _, patch := range patches {
		newSpec.Sources = append(newSpec.Sources, NewSourceFile(patch, SourceFileTypePatch))
	}

	for _, rpm := range builtRpms {
		newSpec.ProvidedRpms = append(newSpec.ProvidedRpms, NewRpmFile(filepath.Join(rpmsDir, rpm)+".rpm"))
	}
	for _, prov := range predictedProvides {
		newProv, err := pkgjson.PackageStringToPackageVer(prov)
		if err != nil {
			logger.Log.Fatalf("Failed to parse package string %s: %s", prov, err)
		}
		newSpec.PredictedProvides = append(newSpec.PredictedProvides, newProv)
	}

	return newSpec
}
