// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package buildconfig

var CurrentBuildConfig BuildConfig

type BuildConfig struct {
	Arch                  string
	DistTag               string
	RpmsDirsByDirtLevel   map[int]string
	RpmsCacheDir          string
	SrpmsDirsByDirtLevel  map[int]string
	InputRepoDir          string
	DoCheck               bool
	MaxDirt               int  // Maximum number of dirt levels, the repo corresponding to this will be "PMC" aka "InputRepoDir"
	AllowCacheForAnyLevel bool // Allow upstream cached pacakges to be used for any dirt level if we don't know how to build a local one
}
