// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package toolkit_types

type SourceFile struct {
	Path string
	Type SourceFileType
}

type SourceFileType int

const (
	SourceFileTypePatch  SourceFileType = iota
	SourceFileTypeSource SourceFileType = iota
)

func NewSourceFile(path string, fileType SourceFileType) *SourceFile {
	return &SourceFile{
		Path: path,
		Type: fileType,
	}
}

type SourceFiles []*SourceFile
