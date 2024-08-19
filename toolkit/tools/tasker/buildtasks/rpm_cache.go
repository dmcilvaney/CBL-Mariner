// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Test for scheduler stuff

package newschedulertasks

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/directory"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/network"
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
	repoMounts, err := docker.MountsForDirtLevel(c.DirtyLevel())
	if err != nil {
		c.TLog(logrus.FatalLevel, "Failed to get mounts for dirt level: %s", err)
	}
	cacheEntry := c.queryRepoListForPackage(repoMounts, true)

	// Do a lookup in the input cache
	if cacheEntry == nil {
		repoMounts = docker.MountForInput()
		cacheEntry = c.queryRepoListForPackage(repoMounts, true)
	}

	// Do a lookup in remote
	if cacheEntry == nil {
		cacheEntry = c.queryRepoListForPackage(nil, false)
	}

	if cacheEntry == nil {
		c.TLog(logrus.FatalLevel, "Failed to find RPM in cache")
	}

	c.SetValue(cacheEntry)
	c.SetDone()
}

func (c *RpmCacheTask) queryRepoListForPackage(repoMounts []docker.DockerOverlay, useLocalRepos bool) *toolkit_types.RpmCache {
	repoPathMap := make(map[int]string)
	for _, overlay := range repoMounts {
		repoPathMap[overlay.Priority] = overlay.Source
	}

	// Check if we have anything in the local cache, and if so what dirt level is it. If localRepos = false, we will be expecting
	// results from arbitrary repos.
	repoEnableString := "local*"
	if !useLocalRepos {
		repoEnableString = "*"
	}
	cmd := "repoquery"
	args := []string{"--disablerepo=*", "--enablerepo=" + repoEnableString, "--whatprovides", c.Capability.PackageVerToPackageString(), "--qf", "PROVIDES_LOOKUP:\t%{name}\t%{version}-%{release}\t%{repoid}"}
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
		c.TLog(logrus.WarnLevel, "Failed to find capability '%s' in local cache", c.Capability.PackageVerToPackageString())
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
		// Get the path to the RPM we found
		args = []string{"--disablerepo=*", "--enablerepo=" + repoEnableString, "--location", bestResult.pkgver.Name, "=", bestResult.pkgver.Version}
		stdout, stderr, err = docker.Run(cmd, args, nil, repoMounts, nil, docker.CacheImageTag, docker.CreateReposAndRun, "", true)
		if err != nil {
			c.TLog(logrus.ErrorLevel, "stderr: %s", stderr)
			c.TLog(logrus.FatalLevel, "Failed to get RPM location: %s", err)
		}
		if useLocalRepos {
			// Extract the path
			pathRegex := regexp.MustCompile(`file:///repos/(\d+)/(.*)`)
			pathResults := pathRegex.FindStringSubmatch(stdout)
			if len(pathResults) != 3 {
				c.TLog(logrus.FatalLevel, "Failed to parse RPM location: %s", stdout)
			}
			// Create the RpmCache object
			repoDir, ok := repoPathMap[bestResult.dirt]
			if !ok {
				c.TLog(logrus.FatalLevel, "Failed to find repo dir for dirt level: %d", bestResult.dirt)
			}
			rpmPath := filepath.Join(repoDir, pathResults[2])
			return toolkit_types.NewRpmCache(rpmPath, true)
		} else {
			// Working with URLs... grab the 1st result and download it to the cache. Explicitly ignore any result that ends with .src.rpm
			urlRegex := regexp.MustCompile(`https?://.*`)
			urlResults := urlRegex.FindAllString(stdout, -1)
			if len(urlResults) == 0 {
				c.TLog(logrus.FatalLevel, "Failed to find RPM URL: %s", stdout)
			}
			for _, urlResult := range urlResults {
				if strings.HasSuffix(urlResult, "src.rpm") {
					continue
				}
				dst := filepath.Join(buildconfig.CurrentBuildConfig.RpmsCacheDir, filepath.Base(urlResult))
				directory.EnsureDirExists(filepath.Dir(dst))
				_, err = network.DownloadFileWithRetry(context.TODO(), urlResult, dst, nil, nil, time.Minute*2)
				if err != nil {
					c.TLog(logrus.FatalLevel, "Failed to download RPM: %s", err)
				}
				return toolkit_types.NewRpmCache(dst, false)
			}
		}
	}
	return nil
}

//docker run --rm -it --mount 'type=volume,dst=/repos/1,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS,upperdir=/tmp/docker-overlay1538210040/upper,workdir=/tmp/docker-overlay1538210040/work"' --mount 'type=volume,dst=/repos/2,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS-dirty/2,upperdir=/tmp/docker-overlay2046217079/upper,workdir=/tmp/docker-overlay2046217079/work"' --mount 'type=volume,dst=/repos/3,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS-dirty/3,upperdir=/tmp/docker-overlay504613287/upper,workdir=/tmp/docker-overlay504613287/work"' --mount 'type=volume,dst=/repos/4,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS-dirty/4,upperdir=/tmp/docker-overlay2879259657/upper,workdir=/tmp/docker-overlay2879259657/work"' --mount 'type=volume,dst=/repos/5,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/tasker_rpm/build/RPMS-dirty/5,upperdir=/tmp/docker-overlay4170428421/upper,workdir=/tmp/docker-overlay4170428421/work"' mcr.microsoft.com/azurelinux/local_builder/cache create_repos_and_run.sh --repodir=/repos/1:1 --repodir=/repos/2:2 --repodir=/repos/3:3 --repodir=/repos/4:4 --repodir=/repos/5:5 bash
