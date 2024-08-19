// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package toolkit_types

type RpmCache struct {
	Path      string
	Available bool
}

func NewRpmCache(path string, available bool) *RpmCache {
	return &RpmCache{
		Path:      path,
		Available: available,
	}
}
