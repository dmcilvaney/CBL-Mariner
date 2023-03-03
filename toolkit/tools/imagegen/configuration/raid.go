// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

import (
	"encoding/json"
	"fmt"
)

// RaidConfig holds the raid configuration information for a software raid disk
type RaidConfig struct {
	ComponentPartIDs []string `json:"ID"`     // PartIDs of the partitions to be used in the raid
	Level            string   `json:"Level"`  // 0, 1, 4, 5, 6, 10
	RaidID           string   `json:"RaidID"` // ID of the raid device used for mdam
}

// IsEmpty returns true if the RaidConfig is empty
func (r *RaidConfig) IsEmpty() bool {
	if r == nil {
		return true
	}
	return len(r.ComponentPartIDs) == 0 && len(r.Level) == 0
}

// IsValid returns an error if the RaidConfig is not valid
func (r *RaidConfig) IsValid() (err error) {
	return nil
}

// UnmarshalJSON Unmarshals a RaidConfig entry
func (r *RaidConfig) UnmarshalJSON(b []byte) (err error) {
	// Use an intermediate type which will use the default JSON unmarshal implementation
	type IntermediateTypeRaidConfig RaidConfig
	err = json.Unmarshal(b, (*IntermediateTypeRaidConfig)(r))
	if err != nil {
		return fmt.Errorf("failed to parse [RaidConfig]: %w", err)
	}

	// Now validate the resulting unmarshaled object
	err = r.IsValid()
	if err != nil {
		return fmt.Errorf("failed to parse [RaidConfig]: %w", err)
	}
	return
}
