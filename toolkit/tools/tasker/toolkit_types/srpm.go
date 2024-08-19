// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package toolkit_types

import (
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
)

type SrpmFile struct {
	// Inputs
	SourceSpec *SpecFile
	NoDeps     bool
	Sources    []*SourceFile

	// Derived
	Path              string
	PredictedProvides []*pkgjson.PackageVer
	BuildRequires     []*pkgjson.PackageVer
}

func NewSrpmFile(sourceSpec *SpecFile, noDeps bool) *SrpmFile {
	sources := make([]*SourceFile, len(sourceSpec.Sources))
	return &SrpmFile{
		SourceSpec: sourceSpec,
		NoDeps:     noDeps,
		Sources:    sources,
	}
}
