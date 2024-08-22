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
	Sources    []*SourceFile

	// Derived
	Path              string
	PredictedProvides []*pkgjson.PackageVer
	BuildRequires     []*pkgjson.PackageVer
}

func NewSrpmFile(sourceSpec *SpecFile) *SrpmFile {
	sources := make([]*SourceFile, len(sourceSpec.Sources))
	return &SrpmFile{
		SourceSpec: sourceSpec,
		Sources:    sources,
	}
}
