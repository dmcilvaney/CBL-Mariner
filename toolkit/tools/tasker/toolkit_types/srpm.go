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
	Requires          []*pkgjson.PackageVer
}

func NewSrpmFile(sourceSpec *SpecFile) *SrpmFile {
	return &SrpmFile{
		SourceSpec: sourceSpec,
		Sources:    sourceSpec.Sources,
	}
}
