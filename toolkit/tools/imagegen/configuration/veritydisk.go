// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// ReadOnlyVerityRoot controls DM-Verity read-only filesystems which will be mounted at startup
// It will create a verity disk from the partition mounted at "/". The verity data is stored as
// part of the image's initramfs.
// - Enable: Enable dm-verity on the root filesystem and add the root hash to the
//     initramfs
// - Name: Custom name for the mounted root (default is "verity_root_fs")
// - ErrorCorrectionEnable: Enable Reed-Solomon forward error correction of read-only data and
//     add the FEC data to the initramfs
// - ErrorCorrectionEncodingRoots: Increase overhead to increase resiliency, default is 2
//     encoding bytes per 255 bytes of real data) giving 0.8% overhead ( RS(255,253) )
//     For a given N (where N = 255 - #Roots), the number of consecutive recoverable blocks is:
//       ceiling(# of 4k blocks in disk / (N)) * (255-N)
//       ie for 2GiB disk: ceiling(524288 / 253) * (255-253) = 2073 * 2 = 4146 blocks = ~16MiB
// - RootHashSignatureEnable: Validate the root hash against a key stored in the kernel's
//     system keyring. The signature file should be called "<Name>.p7" and must be stored in
//     the initramfs. This signature WILL NOT BE included automatically in the initramfs. It must
//     be included via an out of band build step.
// - ValidateOnBoot: Run a validation of the full disk at boot time, normally blocks are validated
//     only as needed. This can take several minutes if the disk is corrupted.
// - VerityErrorBehavior: System behavior when encountering an unrecoverable verity corruption. One
//     of 'ignore', 'restart', 'panic'
// - TmpfsOverlays: Mount these paths as writable overlays backed by a tmpfs in memory. They are
//     discarded on reboot. Overlays should not overlap each other. If a directory is not already
//     present it will be created automatically. Persistant overlays can be created by mounting
//     writable partitions as normal.
// - TmpfsOverlayDebugEnabled: Make the tmpfs overlay mounts easily accessible for debugging
//     purposes. They can be found in /mnt/verity_overlay_debug_tmpfs
type ReadOnlyVerityRoot struct {
	Enable                       bool                `json:"Enable"`
	Name                         string              `json:"Name"`
	ErrorCorrectionEnable        bool                `json:"ErrorCorrectionEnable"`
	ErrorCorrectionEncodingRoots int                 `json:"ErrorCorrectionEncodingRoots"`
	RootHashSignatureEnable      bool                `json:"RootHashSignatureEnable"`
	ValidateOnBoot               bool                `json:"ValidateOnBoot"`
	VerityErrorBehavior          VerityErrorBehavior `json:"VerityErrorBehavior"`
	TmpfsOverlays                []string            `json:"TmpfsOverlays"`
	TmpfsOverlayDebugEnabled     bool                `json:"TmpfsOverlayDebugEnabled"`
}

const (
	defaultName = "verity_root_fs"
	// Default values used for Android's dm-verity FEC, gives 16MiB recovery for a 2GiB disk with 0.8% overhead
	defaultErrorCorrectionEncodingN = 2
	maxErrorCorrectionEncodingRoots = 24
	minErrorCorrectionEncodingRoots = 2
)

var (
	defaultReadOnlyVerityRoot ReadOnlyVerityRoot = ReadOnlyVerityRoot{
		Name:                         defaultName,
		ErrorCorrectionEnable:        true,
		VerityErrorBehavior:          VerityErrorBehaviorRestart,
		ErrorCorrectionEncodingRoots: defaultErrorCorrectionEncodingN,
	}
)

// IsValid returns an error if the ReadOnlyVerityRoot is not valid
func (v *ReadOnlyVerityRoot) IsValid() (err error) {
	// The default disabled config does not generate a default name, only check if we have explicitly
	// enabled the verity config
	if v.Enable && len(v.Name) == 0 {
		return fmt.Errorf("[Name] must not be blank")
	}

	if v.ErrorCorrectionEnable {
		if v.ErrorCorrectionEncodingRoots < minErrorCorrectionEncodingRoots || v.ErrorCorrectionEncodingRoots > maxErrorCorrectionEncodingRoots {
			return fmt.Errorf("verity FEC [ErrorCorrectionEncodingRoots] out of bounds ( %d <= N <= %d), currently %d", minErrorCorrectionEncodingRoots, maxErrorCorrectionEncodingRoots, v.ErrorCorrectionEncodingRoots)
		}
	}

	if err = v.VerityErrorBehavior.IsValid(); err != nil {
		return
	}

	for i, overlayA := range v.TmpfsOverlays {
		for j, overlayB := range v.TmpfsOverlays {
			if i == j {
				continue
			}

			wildcardPattern := filepath.Join(overlayA, "*")
			isNested, err := filepath.Match(wildcardPattern, overlayB)

			if err != nil {
				return fmt.Errorf("failed to check if [TmpfsOverlays] paths are nested (%s)(%s): %w", overlayA, overlayB, err)
			}
			if isNested {
				return fmt.Errorf("failed to validate [TmpfsOverlays], overlays may not overlap each other (%s)(%s)", overlayA, overlayB)
			}
		}
	}

	return
}

// UnmarshalJSON Unmarshals a ReadOnlyVerityRoot entry
func (v *ReadOnlyVerityRoot) UnmarshalJSON(b []byte) (err error) {
	// Use an intermediate type which will use the default JSON unmarshal implementation
	type IntermediateTypeReadOnlyVerityRoot ReadOnlyVerityRoot

	// Populate non-standard default values
	*v = defaultReadOnlyVerityRoot

	err = json.Unmarshal(b, (*IntermediateTypeReadOnlyVerityRoot)(v))
	if err != nil {
		return fmt.Errorf("failed to parse [ReadOnlyVerityRoot]: %w", err)
	}

	// Now validate the resulting unmarshaled object
	err = v.IsValid()
	if err != nil {
		return fmt.Errorf("failed to parse [ReadOnlyVerityRoot]: %w", err)
	}
	return
}
