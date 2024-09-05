// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/jsonutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/network"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"github.com/sirupsen/logrus"
)

type SourceFilesTask struct {
	task.DefaultValueTask[toolkit_types.SourceFiles]
	SpecFile *toolkit_types.SpecFile
	DstDir   string
}

func NewSourceFilesTask(specFile *toolkit_types.SpecFile, dstDir string) *SourceFilesTask {
	return &SourceFilesTask{
		SpecFile: specFile,
		DstDir:   dstDir,
	}
}

func (s *SourceFilesTask) ID() string {
	return s.SpecFile.Path + "_sourcefiles_" + s.DstDir
}

func (s *SourceFilesTask) Name() string {
	return fmt.Sprintf("SOURCES: %s", filepath.Base(s.SpecFile.Path))
}

func (s *SourceFilesTask) Execute() {
	allSources, err := s.hydrateAllFiles(s.SpecFile, s.DstDir)
	if err != nil {
		s.TLog(logrus.FatalLevel, "Failed to hydrate source files: %s", err)
	}

	s.SetValue(toolkit_types.SourceFiles(allSources))
	s.SetDone()
}

func (s *SourceFilesTask) hydrateAllFiles(specFile *toolkit_types.SpecFile, dstDir string) ([]*toolkit_types.SourceFile, error) {
	fileHydrationState := make(map[string]bool)
	needSig := make(map[string]bool)
	sigLookup, err := loadSignatureData(specFile.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load signature data: %s", err)
	}

	specDir := filepath.Dir(specFile.Path)
	for _, source := range specFile.Sources {
		fileHydrationState[filepath.Base(source.Path)] = false
		needSig[filepath.Base(source.Path)] = (source.Type != toolkit_types.SourceFileTypePatch)
	}

	// Try local dir first
	err = tryToHydrateFromLocalSource(fileHydrationState, specDir, dstDir, sigLookup, needSig)
	if err != nil {
		return nil, fmt.Errorf("failed to hydrate source files from local dir: %s", err)
	}

	// Try remote dir
	err = hydrateFromRemoteSource(context.Background(), fileHydrationState, dstDir, sigLookup)
	if err != nil {
		return nil, fmt.Errorf("failed to hydrate source files from remote dir: %s", err)
	}

	for f, got := range fileHydrationState {
		if !got {
			return nil, fmt.Errorf("failed to hydrate source file: %s", f)
		}
	}

	return specFile.Sources, nil
}

func tryToHydrateFromLocalSource(fileHydrationState map[string]bool, localSrcDir, dstDir string, sigLookup map[string]string, needsSig map[string]bool) (err error) {
	return filepath.Walk(localSrcDir, func(path string, info os.FileInfo, walkErr error) (internalErr error) {
		if walkErr != nil {
			return walkErr
		}

		isFile, _ := file.IsFile(path)
		if !isFile {
			return nil
		}

		fileName := filepath.Base(path)

		isHydrated, fileRequiredBySpec := fileHydrationState[fileName]
		if !fileRequiredBySpec {
			return nil
		}

		if isHydrated {
			logger.Log.Warnf("Duplicate matching file found at (%s), skipping", path)
			return nil
		}

		if needsSig[path] {
			internalErr = validateSignature(path, sigLookup)
			if internalErr != nil {
				return internalErr
			}
		}

		internalErr = file.Copy(path, filepath.Join(dstDir, fileName))
		if internalErr != nil {
			return internalErr
		}

		logger.Log.Debugf("Hydrated (%s) from (%s)", fileName, path)

		fileHydrationState[fileName] = true
		return nil
	})
}

func hydrateFromRemoteSource(ctx context.Context, fileHydrationState map[string]bool, dstDir string, sigLookup map[string]string) (err error) {
	errPackerCancelReceived := fmt.Errorf("packer cancel signal received")

	for fileName, alreadyHydrated := range fileHydrationState {
		if alreadyHydrated {
			continue
		}

		destinationFile := filepath.Join(dstDir, fileName)

		url := network.JoinURL(buildconfig.CurrentBuildConfig.SourceUrl, fileName)

		cancelled, internalErr := network.DownloadFileWithRetry(ctx, url, destinationFile, nil, nil, network.DefaultTimeout)

		// We may intentionally fail early due to a cancellation signal, stop immediately if that is the case.
		if cancelled {
			err = errPackerCancelReceived
			return
		}

		if internalErr != nil {
			logger.Log.Errorf("Failed to download (%s). Error: %s.", url, internalErr)
			continue
		}

		internalErr = validateSignature(destinationFile, sigLookup)
		if internalErr != nil {
			logger.Log.Errorf("Signature validation for (%s) failed. Error: %s.", destinationFile, internalErr)

			// If the delete fails, just warn as there will be another cleanup
			// attempt when exiting the program.
			internalErr = os.Remove(destinationFile)
			if internalErr != nil {
				logger.Log.Warnf("Failed to delete file (%s) after signature validation failure. Error: %s.", destinationFile, internalErr)
			}
			continue
		}

		fileHydrationState[fileName] = true
		logger.Log.Debugf("Hydrated (%s) from (%s)", fileName, url)
	}

	return nil
}

func validateSignature(path string, sigLookup map[string]string) (err error) {
	fileName := filepath.Base(path)
	expectedSignature, found := sigLookup[fileName]
	if !found {
		err = fmt.Errorf("no signature for file (%s) found. full path is (%s)", fileName, path)
		return
	}

	newSignature, err := file.GenerateSHA256(path)
	if err != nil {
		return
	}

	if !strings.EqualFold(expectedSignature, newSignature) {
		return fmt.Errorf("file (%s) has mismatching signature: expected (%s) - actual (%s)", path, expectedSignature, newSignature)
	}

	return
}

func loadSignatureData(specFilePath string) (map[string]string, error) {
	const (
		specSuffix          = ".spec"
		signatureFileSuffix = "signatures.json"
	)

	specName := strings.TrimSuffix(filepath.Base(specFilePath), specSuffix)
	signatureFileName := fmt.Sprintf("%s.%s", specName, signatureFileSuffix)
	signatureFileDirPath := filepath.Dir(specFilePath)

	return readSignatures(filepath.Join(signatureFileDirPath, signatureFileName))
}

func readSignatures(signaturesFilePath string) (readSignatures map[string]string, err error) {
	type fileSignaturesWrapper struct {
		FileSignatures map[string]string `json:"Signatures"`
	}

	var signaturesWrapper fileSignaturesWrapper
	signaturesWrapper.FileSignatures = make(map[string]string)

	err = jsonutils.ReadJSONFile(signaturesFilePath, &signaturesWrapper)
	if err != nil {
		if os.IsNotExist(err) {
			// Non-fatal as some SPECs may not have sources
			logger.Log.Debugf("The signatures file (%s) doesn't exist, will not pre-populate signatures.", signaturesFilePath)
			err = nil
		} else {
			logger.Log.Errorf("Failed to read the signatures file (%s): %v.", signaturesFilePath, err)
		}
	}

	return signaturesWrapper.FileSignatures, err
}
