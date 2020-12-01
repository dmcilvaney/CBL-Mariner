// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SystemConfig defines how each system present on the image is supposed to be configured.
type SystemConfig struct {
	IsDefault          bool                `json:"IsDefault"`
	BootType           string              `json:"BootType"`
	Hostname           string              `json:"Hostname"`
	Name               string              `json:"Name"`
	PackageLists       []string            `json:"PackageLists"`
	KernelOptions      map[string]string   `json:"KernelOptions"`
	KernelCommandLine  KernelCommandLine   `json:"KernelCommandLine"`
	AdditionalFiles    map[string]string   `json:"AdditionalFiles"`
	PartitionSettings  []PartitionSetting  `json:"PartitionSettings"`
	PostInstallScripts []PostInstallScript `json:"PostInstallScripts"`
	Groups             []Group             `json:"Groups"`
	Users              []User              `json:"Users"`
	Encryption         RootEncryption      `json:"Encryption"`
	ReadOnlyVerityRoot ReadOnlyVerityRoot  `json:"ReadOnlyVerityRoot"`
}

// IsValid returns an error if the SystemConfig is not valid
func (s *SystemConfig) IsValid() (err error) {
	// IsDefault must be validated by a parent struct

	// Validate BootType

	// Validate HostName

	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("missing [Name] field")
	}

	if len(s.PackageLists) == 0 {
		return fmt.Errorf("system configuration must provide at least one package list inside the [PackageLists] field")
	}
	// Additional package list validation must be done via the imageconfigvalidator tool since there is no guranatee that
	// the paths are valid at this point.

	// Enforce that any non-rootfs configuration has a default kernel.
	if len(s.PartitionSettings) != 0 {
		// Ensure that default option is always present
		if _, ok := s.KernelOptions["default"]; !ok {
			return fmt.Errorf("system configuration must always provide default kernel inside the [KernelOptions] field; remember that kernels are FORBIDDEN from appearing in any of the [PackageLists]")
		}
		// Ensure that non-comment options are not blank
		for name, kernelName := range s.KernelOptions {
			// Skip comments
			if name[0] == '_' {
				continue
			}
			if strings.TrimSpace(kernelName) == "" {
				return fmt.Errorf("empty kernel entry found in the [KernelOptions] field (%s); remember that kernels are FORBIDDEN from appearing in any of the [PackageLists]", name)
			}
		}
	}

	mountPointUsed := make(map[string]bool)
	for _, partitionSetting := range s.PartitionSettings {
		if err = partitionSetting.IsValid(); err != nil {
			return fmt.Errorf("invalid [PartitionSettings]: %w", err)
		}
		if mountPointUsed[partitionSetting.MountPoint] {
			return fmt.Errorf("invalid [PartitionSettings]: duplicate mount point found at '%s'", partitionSetting.MountPoint)
		}
		mountPointUsed[partitionSetting.MountPoint] = true
	}

	if s.ReadOnlyVerityRoot.Enable {
		if !mountPointUsed["/"] {
			return fmt.Errorf("invalid [ReadOnlyVerityRoot]: must have a partition mounted at '/'")
		}
		if !mountPointUsed["/boot"] {
			return fmt.Errorf("invalid [ReadOnlyVerityRoot]: must have a separate partition mounted at '/boot'")
		}
		if s.Encryption.Enable {
			return fmt.Errorf("invalid [ReadOnlyVerityRoot]: verity root currently does not support root encryption")
		}
	}
	if err = s.ReadOnlyVerityRoot.IsValid(); err != nil {
		return fmt.Errorf("invalid [ReadOnlyVerityRoot]: %w", err)
	}

	if err = s.KernelCommandLine.IsValid(); err != nil {
		return fmt.Errorf("invalid [KernelCommandLine]: %w", err)
	}

	//Validate PostInstallScripts
	//Validate Groups
	//Validate Users
	//Validate Encryption

	return
}

// UnmarshalJSON Unmarshals a Disk entry
func (s *SystemConfig) UnmarshalJSON(b []byte) (err error) {
	// Use an intermediate type which will use the default JSON unmarshal implementation
	type IntermediateTypeSystemConfig SystemConfig
	err = json.Unmarshal(b, (*IntermediateTypeSystemConfig)(s))
	if err != nil {
		return fmt.Errorf("failed to parse [SystemConfig]: %w", err)
	}

	// Now validate the resulting unmarshaled object
	err = s.IsValid()
	if err != nil {
		return fmt.Errorf("failed to parse [SystemConfig]: %w", err)
	}
	return
}
