// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package toolkit_types

import (
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/rpm"
)

type RpmFile struct {
	Path         string
	Capibilities []*pkgjson.PackageVer
}

func NewRpmFile(path string) *RpmFile {
	return &RpmFile{
		Path: path,
	}
}

func NewRpmFileWithCapabilitiesFromRealFile(path string) *RpmFile {
	capabilities, err := rpm.QueryRPMProvides2(path)
	if err != nil {
		return nil
	}
	return &RpmFile{
		Path:         path,
		Capibilities: capabilities,
	}
}
