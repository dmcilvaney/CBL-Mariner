// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package toolkit_types

import (
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
)

// TODO THis is just a fake wrapper around the old json tool's data
var GlobalSpecDataDB *SpecDataDB

type SpecDataDB struct {
	specDataDB *pkgjson.PackageRepo
}

func NewSpecDataDB(input string) {
	GlobalSpecDataDB = &SpecDataDB{&pkgjson.PackageRepo{}}
	err := GlobalSpecDataDB.specDataDB.ParsePackageJSON(input)
	if err != nil {
		logger.Log.Panic(err)
	}
}

// Super basic lookup function, returns the 1st package that matches the capability
func (*SpecDataDB) LookupRpmCapabilityTask(capability *pkgjson.PackageVer) *pkgjson.Package {
	pkgName := capability.Name
	versionInterval, err := capability.Interval()
	if err != nil {
		logger.Log.Panic(err)
	}

	for _, pkg := range GlobalSpecDataDB.specDataDB.Repo {
		provides := pkg.Provides
		if provides.Name == pkgName {
			providesInterval, err := provides.Interval()
			if err != nil {
				logger.Log.Panic(err)
			}
			if providesInterval.Satisfies(&versionInterval) {
				return pkg
			}
		}
	}

	return nil
}
