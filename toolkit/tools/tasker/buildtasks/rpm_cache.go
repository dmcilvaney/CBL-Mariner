// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/directory"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/docker"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/task"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/toolkit_types"
	"github.com/sirupsen/logrus"
)

type RpmCacheTask struct {
	task.DefaultValueTask[*toolkit_types.RpmCache]
	// Input
	allowableDirt int
	// Input/Output
	Capability *pkgjson.PackageVer
}

func NewRpmCacheTask(capability *pkgjson.PackageVer, allowableDirt int) *RpmCacheTask {
	newRpmCacheTask := &RpmCacheTask{
		Capability:    capability,
		allowableDirt: allowableDirt,
	}
	newRpmCacheTask.SetInfo(
		"CACHE_"+capability.String(),
		fmt.Sprintf("CACHE: %s", capability.String()),
		allowableDirt,
	)

	return newRpmCacheTask
}

func (c *RpmCacheTask) Execute() {
	repoMounts, err := docker.MountsForDirtLevel(c.DirtyLevel(), buildconfig.CurrentBuildConfig.AllowCacheForAnyLevel)
	if err != nil {
		c.TLog(logrus.FatalLevel, "Failed to get mounts for dirt level: %s", err)
	}
	cacheEntry := c.DnfCacheDownloader(repoMounts, true)

	// Do a lookup in the input cache
	if cacheEntry == nil {
		repoMounts = docker.MountForInput()
		cacheEntry = c.DnfCacheDownloader(repoMounts, true)
	}

	// Do a lookup in remote
	if cacheEntry == nil {
		repoMounts := docker.AllMounts()
		if err != nil {
			c.TLog(logrus.FatalLevel, "Failed to get mounts for dirt level: %s", err)
		}
		cacheEntry = c.DnfCacheDownloader(repoMounts, false)
	}

	if cacheEntry == nil {
		c.TLog(logrus.FatalLevel, "Failed to find RPM in cache")
	} else {
		c.TLog(logrus.InfoLevel, "Found RPM: %s", cacheEntry.Path)
	}

	c.SetValue(cacheEntry)
	c.SetDone()
}

func (c *RpmCacheTask) getCanonicalPackage(searchCap *pkgjson.PackageVer, repoMounts []docker.DockerOverlay, repoEnableString string, useLocalRepos bool) *pkgjson.PackageVer {
	repoPathMap := make(map[int]string)
	for _, overlay := range repoMounts {
		repoPathMap[overlay.Priority] = overlay.Source
	}

	// Check if we have anything in the local cache, and if so what dirt level is it. If localRepos = false, we will be expecting
	// results from arbitrary repos.
	cmd := "repoquery"
	args := []string{"--disablerepo=*", "--enablerepo=" + repoEnableString, "--qf", "PROVIDES_LOOKUP:\t%{name}\t%{evr}\t%{repoid}"}
	args = append(args, "--whatprovides")
	args = append(args, searchCap.PackageVerToPackageString())
	stdout, stderr, err := docker.Run(cmd, args, nil, repoMounts, nil, docker.CacheImageTag, docker.CreateReposAndRun, "", true)
	if err != nil {
		c.TLog(logrus.ErrorLevel, "stderr: %s", stderr)
		c.TLog(logrus.FatalLevel, "Failed to cache RPMs: %s", err)
	}

	// Find all the entires, and extract the dirt level (local-#)
	cacheResultRegex := regexp.MustCompile(`PROVIDES_LOOKUP:\t(.*)\t(.*)\tlocal-(\d+)`)
	if !useLocalRepos {
		cacheResultRegex = regexp.MustCompile(`PROVIDES_LOOKUP:\t(.*)\t(.*)\t(.*)`)
	}

	cacheResults := cacheResultRegex.FindAllStringSubmatch(stdout, -1)
	if len(cacheResults) == 0 {
		c.TLog(logrus.WarnLevel, "Failed to find capability '%s' in local cache", searchCap.PackageVerToPackageString())
		return nil
	}

	type cacheResult struct {
		pkgver *pkgjson.PackageVer
		dirt   int
	}
	bestResult := cacheResult{dirt: 9999}
	if useLocalRepos {
		for _, res := range cacheResults {
			if len(res) != 4 {
				c.TLog(logrus.FatalLevel, "Failed to parse cache result: %s", res)
			}
			dirt, err := strconv.Atoi(res[3])
			if err != nil {
				c.TLog(logrus.FatalLevel, "Failed to parse dirt level: %s", err)
			}
			if dirt < bestResult.dirt {
				pkg, err := pkgjson.PackageStringToPackageVer(fmt.Sprintf("%s = %s", res[1], res[2]))
				if err != nil {
					c.TLog(logrus.FatalLevel, "Failed to parse package: %s", err)
				}
				bestResult = cacheResult{pkgver: pkg, dirt: dirt}
			}
		}
	} else {
		for _, res := range cacheResults {
			if len(res) != 4 {
				c.TLog(logrus.WarnLevel, "Failed to parse cache result: %s", res)
				continue
			}
			pkg, err := pkgjson.PackageStringToPackageVer(fmt.Sprintf("%s = %s", res[1], res[2]))
			if err != nil {
				c.TLog(logrus.WarnLevel, "Failed to parse package: %s", err)
				continue
			}
			bestResult = cacheResult{pkgver: pkg, dirt: c.allowableDirt}
			break
		}
	}
	if bestResult.dirt <= c.allowableDirt {
		return bestResult.pkgver
	} else {
		return nil
	}
}

// func (c *RpmCacheTask) queryRepoListForPackage(repoMounts []docker.DockerOverlay, useLocalRepos bool) *toolkit_types.RpmCache {
// 	l := task.AcquireTaskLimiter(1)
// 	defer l.Release()

// 	canonicalPackage := c.getCanonicalPackage(c.Capability, repoMounts, useLocalRepos)

// 	// Get the path to the RPM we found
// 	args := []string{"--disablerepo=*", "--enablerepo=" + repoEnableString, "--location", bestResult.pkgver.Name, "=", bestResult.pkgver.Version}
// 	stdout, stderr, err = docker.Run(cmd, args, nil, repoMounts, nil, docker.CacheImageTag, docker.CreateReposAndRun, "", true)
// 	if err != nil {
// 		c.TLog(logrus.ErrorLevel, "stderr: %s", stderr)
// 		c.TLog(logrus.FatalLevel, "Failed to get RPM location: %s", err)
// 	}
// 	if useLocalRepos {
// 		// Extract the path
// 		pathRegex := regexp.MustCompile(`file:///repos/(\d+)/(.*)`)
// 		pathResults := pathRegex.FindStringSubmatch(stdout)
// 		if len(pathResults) != 3 {
// 			c.TLog(logrus.FatalLevel, "Failed to parse RPM location: %s", stdout)
// 		}
// 		// Create the RpmCache object
// 		repoDir, ok := repoPathMap[bestResult.dirt]
// 		if !ok {
// 			c.TLog(logrus.FatalLevel, "Failed to find repo dir for dirt level: %d", bestResult.dirt)
// 		}
// 		rpmPath := filepath.Join(repoDir, pathResults[2])
// 		return toolkit_types.NewRpmCache(rpmPath, true)
// 	} else {
// 		// Working with URLs... grab the 1st result and download it to the cache. Explicitly ignore any result that ends with .src.rpm
// 		urlRegex := regexp.MustCompile(`https?://.*`)
// 		urlResults := urlRegex.FindAllString(stdout, -1)
// 		if len(urlResults) == 0 {
// 			c.TLog(logrus.FatalLevel, "Failed to find RPM URL: %s", stdout)
// 		}
// 		for _, urlResult := range urlResults {
// 			if strings.HasSuffix(urlResult, "src.rpm") {
// 				continue
// 			}
// 			dst := filepath.Join(buildconfig.CurrentBuildConfig.RpmsCacheDir, filepath.Base(urlResult))
// 			directory.EnsureDirExists(filepath.Dir(dst))
// 			_, err = network.DownloadFileWithRetry(context.TODO(), urlResult, dst, nil, nil, time.Minute*2)
// 			if err != nil {
// 				c.TLog(logrus.FatalLevel, "Failed to download RPM: %s", err)
// 			}
// 			return toolkit_types.NewRpmCache(dst, false)
// 		}
// 	}

// 	return nil
// }

func (c *RpmCacheTask) DnfCacheDownloader(repoMounts []docker.DockerOverlay, useLocalRepos bool) *toolkit_types.RpmCache {
	c.ClaimLimit(1)
	defer c.ReleaseLimit()

	// Check if we have anything in the local cache, and if so what dirt level is it. If localRepos = false, we will be expecting
	// results from arbitrary repos.
	repoEnableString := "local*"
	if !useLocalRepos {
		repoEnableString = "*"
	}

	canonicalPackage := c.getCanonicalPackage(c.Capability, repoMounts, repoEnableString, useLocalRepos)
	if canonicalPackage == nil {
		c.TLog(logrus.WarnLevel, "Failed to find capability '%s' in local cache", c.Capability.PackageVerToPackageString())
		return nil
	} else {
		c.TLog(logrus.InfoLevel, "Translated capability '%s' to '%s'", c.Capability.PackageVerToPackageString(), canonicalPackage.PackageVerToPackageString())
	}

	cmd := "dnf"
	tempDstDir := c.GetWorkDir()
	defer os.RemoveAll(tempDstDir)

	mount := docker.DockerMount{
		Source: tempDstDir,
		Dest:   "/download",
	}
	//args := []string{"--disablerepo=*", "--enablerepo=" + repoEnableString, "--whatprovides", c.Capability.PackageVerToPackageString(), "--qf", "PROVIDES_LOOKUP:\t%{name}\t%{version}-%{release}\t%{repoid}"}
	args := []string{"download", "--assumeyes", "--downloaddir", "/download", "--alldeps", "--resolve", "--disablerepo=*", "--enablerepo=" + repoEnableString}
	args = append(args, canonicalPackage.PackageVerToPackageString())
	_, stderr, err := docker.Run(cmd, args, &mount, repoMounts, nil, docker.CacheImageTag, docker.CreateReposAndRun, "", true)
	if err != nil {
		c.TLog(logrus.ErrorLevel, "stderr: %s", stderr)
		c.TLog(logrus.ErrorLevel, "Failed to cache RPMs: %s", err)
		return nil
	}

	dstDir := buildconfig.CurrentBuildConfig.RpmsCacheDir

	paths, err := c.syncTransferFiles(tempDstDir, dstDir)
	if len(paths) == 0 {
		// debug
		_, _ = c.syncTransferFiles(tempDstDir, dstDir)
	}
	if err != nil || len(paths) == 0 {
		c.TLog(logrus.FatalLevel, "Failed to sync transfer files: %s", err)
	}

	c.TLog(logrus.InfoLevel, "Downloaded %d RPMs to %s", len(paths), buildconfig.CurrentBuildConfig.RpmsCacheDir)

	return toolkit_types.NewRpmCache(buildconfig.CurrentBuildConfig.RpmsCacheDir+"/MULTI_CACHE", false)
}

var fileMutex = &sync.Mutex{}

func (c *RpmCacheTask) syncTransferFiles(src, dst string) ([]string, error) {
	const rpmExtension = ".rpm"

	fileMutex.Lock()
	defer fileMutex.Unlock()

	err := directory.EnsureDirExists(filepath.Dir(dst))
	if err != nil {
		c.TLog(logrus.FatalLevel, "Failed to create dir: %s", err)
	}

	finalPaths := []string{}
	err = filepath.Walk(src, func(path string, info os.FileInfo, fileErr error) (err error) {
		if fileErr != nil {
			return fileErr
		}

		// Only copy regular files (not unix sockets, directories, links, ...)
		if !info.Mode().IsRegular() {
			return nil
		}

		if !strings.HasSuffix(path, rpmExtension) {
			return nil
		}

		dstPath := filepath.Join(dst, filepath.Base(path))
		finalPaths = append(finalPaths, dstPath)

		exists, err := file.PathExists(dstPath)
		if err != nil {
			return err
		}
		if exists {
			// If the file already exists, we don't need to copy it
			return nil
		}

		err = file.Copy(path, dstPath)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return finalPaths, nil
}
