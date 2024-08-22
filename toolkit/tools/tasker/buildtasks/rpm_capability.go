// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"fmt"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/rpm"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"github.com/sirupsen/logrus"
)

type RpmCapibilityTask struct {
	task.DefaultValueTask[*toolkit_types.RpmCapibility]
	// In
	capability *pkgjson.PackageVer
	// Out
	mappedPackage *toolkit_types.RpmFile
	runtimeDeps   []*toolkit_types.RpmCapibility
}

var CapabilityDb = make(map[pkgjson.PackageVer]*RpmCapibilityTask)

func NewRpmCapibilityTask(capability *pkgjson.PackageVer, dirtLevel int) *RpmCapibilityTask {
	newRpmCapibilityTask := &RpmCapibilityTask{
		capability: capability,
	}
	newRpmCapibilityTask.SetInfo(
		fmt.Sprintf("CAP%d_%s", dirtLevel, capability.String()),
		fmt.Sprintf("CAP: %s", capability.String()),
		dirtLevel,
	)

	return newRpmCapibilityTask
}

// Get the file path of the rpm file, then query the runtime dependencies of the rpm file, and then ensure we have the deps available.
func (r *RpmCapibilityTask) Execute() {
	msg := fmt.Sprintf("RPM Capability: %s", r.capability.String())

	specDB := r.AddDependency(
		NewLoadSpecDataTask(),
	).(*LoadSpecDataTask).Value()

	// Find in our database to see what we need to build
	capLookup := specDB.LookupRpmCapabilityTask(r.capability)

	// See if we can just use an existing capability with lower dirt
	var existingCap *RpmCapibilityTask = nil
	for dirtLevel := 0; dirtLevel < r.DirtyLevel(); dirtLevel++ {
		checkForExisting := r.AddDependency(
			NewRpmCapibilityTask(r.capability, dirtLevel),
		)
		// Found an existing capability that doesn't create a circular dependency
		if checkForExisting != nil {
			existingCap = checkForExisting.(*RpmCapibilityTask)
			r.cloneCapability(existingCap.Value())
			break
		}
	}

	if existingCap == nil {
		if capLookup == nil || r.DirtyLevel() >= buildconfig.CurrentBuildConfig.MaxDirt {
			// TODO, use the cache lookup via capability to find an rpm that might provide this... We don't want to just blindly use PMC
			r.TLog(logrus.WarnLevel, "Failed to find package for capability in DB, need to do lookup!")
			r.findCachedCapability(r.DirtyLevel() + 1)
		} else {
			builtSpecTask := r.AddDependency(
				NewBuildSpecFileTask(capLookup.SpecPath, r.DirtyLevel(), buildconfig.CurrentBuildConfig),
			)
			// Nil means circular dependency... we can queue up a dirty copy, or if its too dirty just grab from the cache.
			// If we reach the max dirt level, we just grab from the cache always.
			allowableDirtLevel := r.DirtyLevel() + 1
			if builtSpecTask == nil && allowableDirtLevel < buildconfig.CurrentBuildConfig.MaxDirt {
				r.TLog(logrus.InfoLevel, "Queueing up a dirty (%d) copy of the spec file", allowableDirtLevel)
				builtSpecTask = r.AddDependency(
					NewBuildSpecFileTask(capLookup.SpecPath, allowableDirtLevel, buildconfig.CurrentBuildConfig),
				)
				if builtSpecTask == nil {
					r.TLog(logrus.FatalLevel, "Failed to build spec file")
				}
			}

			if builtSpecTask != nil {
				builtSpec := builtSpecTask.(*BuildSpecFileTask).Value()
				r.assignBuiltSpec(&builtSpec)
			} else {
				r.TLog(logrus.InfoLevel, "Unable to queue up a dirty copy of the spec file, checking cache with dirt level %d", allowableDirtLevel)
				// We are too dirty, just grab from the cache
				r.findCachedCapability(allowableDirtLevel)
			}
		}
	}

	// Make sure we have the runtime dependencies
	r.collectRuntimeDeps()

	r.TLog(logrus.InfoLevel, msg)
	r.SetValue(toolkit_types.NewRpmCapibility(r.capability, r.mappedPackage))
	r.SetDone()
}

func (r *RpmCapibilityTask) findCachedCapability(allowableDirt int) {
	RpmCacheEntry := r.AddDependency(
		NewRpmCacheTask(r.capability, allowableDirt),
	).(*RpmCacheTask).Value()

	if RpmCacheEntry == nil {
		r.TLog(logrus.FatalLevel, "Failed to find package for capability")
	} else {
		r.mappedPackage = toolkit_types.NewRpmFileWithCapabilitiesFromRealFile(RpmCacheEntry.Path)
	}
}

func (r *RpmCapibilityTask) assignBuiltSpec(builtSpec *toolkit_types.SpecFile) {
	// Scan the build RPMs to find our match
	requiredVersion, err := r.capability.Interval()
	if err != nil {
		r.TLog(logrus.FatalLevel, "Failed to parse capability interval")
	}
	for _, rpm := range builtSpec.ProvidedRpms {
		for _, cap := range rpm.Capibilities {
			providedVersion, err := cap.Interval()
			if err != nil {
				r.TLog(logrus.FatalLevel, "Failed to parse capability interval")
			}
			if cap.Name == r.capability.Name {
				if providedVersion.Satisfies(&requiredVersion) {
					r.mappedPackage = rpm
					break
				}
			}
		}
	}
	if r.mappedPackage == nil {
		r.TLog(logrus.FatalLevel, "Failed to find package for capability")
	}
}

func (r *RpmCapibilityTask) collectRuntimeDeps() {
	deps, err := rpm.QueryRPMRequires2(r.mappedPackage.Path)
	if err != nil {
		r.TLog(logrus.FatalLevel, "Failed to query rpm requires")
	}
	depTasks := make([]*RpmCapibilityTask, 0)
	for _, dep := range deps {
		if !strings.HasPrefix(dep.Name, "rpmlib") {
			depTask := r.AddDependency(
				NewRpmCapibilityTask(dep, r.DirtyLevel()),
			).(*RpmCapibilityTask)
			depTasks = append(depTasks, depTask)
		}
	}
	for _, dep := range depTasks {
		r.runtimeDeps = append(r.runtimeDeps, dep.Value())
	}
}

func (r *RpmCapibilityTask) cloneCapability(cap *toolkit_types.RpmCapibility) {
	r.mappedPackage = cap.MappedPackage
	r.runtimeDeps = nil
}
