// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package toolkit_types

import "github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"

type RpmCapibility struct {
	DesiredCapability *pkgjson.PackageVer
	MappedPackage     *RpmFile
}

func NewRpmCapibility(cap *pkgjson.PackageVer, mapped *RpmFile) *RpmCapibility {
	return &RpmCapibility{
		DesiredCapability: cap,
		MappedPackage:     mapped,
	}
}
