// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package repoutils

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safechroot"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/timestamp"
	"github.com/sirupsen/logrus"
)

const (
	chrootRepoDir       = "/etc/yum.repos.d/"
	dnf5PackageName     = "dnf5"
)

// GetAllRepoData returns a map of package names to URLs for all packages
// available in the given repos. It uses a chroot to run repoquery.
func GetAllRepoData(repoURLs, repoFiles []string, workerTar, buildDir, repoUrlsFile string) (namesToURLs map[string]string, err error) {
	const (
		leaveChrootOnDisk = false
	)
	timestamp.StartEvent("pull available package data from repos", nil)
	defer timestamp.StopEvent(nil)

	queryChroot, err := createChroot(workerTar, buildDir, repoFiles, leaveChrootOnDisk)
	if err != nil {
		err = fmt.Errorf("failed to create chroot:\n%w", err)
		return nil, err
	}
	defer queryChroot.Close(leaveChrootOnDisk)

	namesToURLs = make(map[string]string)
	for _, repoURL := range repoURLs {
		// Use the chroot to query each repo for the packages it contains
		var repoData map[string]string
		err = queryChroot.Run(func() (chrootErr error) {
			repoData, chrootErr = getPackageRepoPathsFromUrl(repoURL)
			return chrootErr
		})
		if err != nil {
			return nil, err
		}
		for k, v := range repoData {
			namesToURLs[k] = v
		}
	}

	// Use the chroot to query each repo for the packages it contains
	for _, repoFile := range repoFiles {
		// Replace the existing repo file (if it exists) with the one that we want to query
		logger.Log.Infof("Will query package data from %s", repoFile)
		err = copyRepoFile(repoFile, queryChroot)
		if err != nil {
			err = fmt.Errorf("failed to add files to chroot:\n%w", err)
			return
		}
	}

	// Only run repoquery if we have repo files to query. --enablerepo=* will not work if there are no repo files
	// and will return "Error: Unknown repo: '*'"
	if len(repoFiles) > 0 {
		var repoFileData map[string]string
		err = queryChroot.Run(func() (chrootErr error) {
			repoFileData, chrootErr = getPackageRepoUrlsFromRepoFiles()
			return chrootErr
		})
		if err != nil {
			return nil, err
		}
		for k, v := range repoFileData {
			namesToURLs[k] = v
		}
	}

	if repoUrlsFile != "" {
		allURLs := make([]string, 0, len(namesToURLs))
		for _, url := range namesToURLs {
			allURLs = append(allURLs, url)
		}
		err = file.WriteLines(allURLs, repoUrlsFile)
	}

	return
}

// createChroot creates a network-enabled chroot to run repoquery in. The caller is expected to call Close() on the
// returned chroot unless an error is returned, in which case the chroot will be closed by this function.
// The created chroot will attempt to use any additional repos provided in repoFiles if the default repos fail to
// install the repoquery package. Once complete ALL repos will be removed from the chroot.
func createChroot(workerTar, chrootDir string, repoFiles []string, leaveChrootOnDisk bool) (queryChroot *safechroot.Chroot, err error) {
	timestamp.StartEvent("creating repoquery chroot", nil)
	defer timestamp.StopEvent(nil)
	logger.Log.Info("Creating chroot for repoquery")

	queryChroot = safechroot.NewChroot(chrootDir, false)
	err = queryChroot.Initialize(workerTar, nil, nil, true)
	if err != nil {
		err = fmt.Errorf("failed to initialize chroot:\n%w", err)
		return
	}

	defer func() {
		if err != nil {
			closeErr := queryChroot.Close(leaveChrootOnDisk)
			if closeErr != nil {
				logger.Log.Errorf("Failed to close chroot, err: %s", closeErr)
			}
		}
	}()

	// We will need network to install the repoquery package
	files := []safechroot.FileToCopy{
		{Src: "/etc/resolv.conf", Dest: "/etc/resolv.conf"},
	}
	err = queryChroot.AddFiles(files...)
	if err != nil {
		err = fmt.Errorf("failed to add files to chroot:\n%w", err)
		return
	}

	// Install the repoquery package from upstream, then clean up any existing repos
	// For preference we start with the default repos from the chroot, then add extra
	// ones if that fails.
	logger.Log.Infof("Installing '%s' package to get 'dnf5 repoquery' command", dnf5PackageName)
	err = installDnf5(queryChroot, nil)
	if len(repoFiles) > 0 && err != nil {
		logger.Log.Warnf("Failed to install '%s' package, trying with additional repos.", dnf5PackageName)
		logger.Log.Warnf("Initial error: %s", err.Error())
		err = installDnf5(queryChroot, repoFiles)
	}
	if err != nil {
		err = fmt.Errorf("failed to install '%s' package:\n%w", dnf5PackageName, err)
		return
	}

	// Remove all existing repos, we will be adding the repo files we want to query later
	err = queryChroot.Run(func() error {
		return os.RemoveAll("/etc/yum.repos.d")
	})
	if err != nil {
		return queryChroot, fmt.Errorf("failed to remove existing repos:\n%w", err)
	}

	return
}

func copyRepoFile(repoSrcPath string, chroot *safechroot.Chroot) (err error) {
	destFile := path.Join(chrootRepoDir, path.Base(repoSrcPath))
	chrootRepoFile := []safechroot.FileToCopy{
		{Src: repoSrcPath, Dest: destFile},
	}
	err = chroot.AddFiles(chrootRepoFile...)
	if err != nil {
		return fmt.Errorf("failed to add files to chroot:\n%w", err)
	}
	return nil
}

func installDnf5(chroot *safechroot.Chroot, additionalRepos []string) (err error) {
	const rootDir = "/"
	// Add any additional repos to the chroot
	for _, repoFile := range additionalRepos {
		err = copyRepoFile(repoFile, chroot)
		if err != nil {
			return fmt.Errorf("failed to copy repo file to chroot:\n%w", err)
		}
	}

	err = chroot.Run(func() error {
		_, chrootErr := installutils.TdnfInstall(dnf5PackageName, rootDir)
		if chrootErr != nil {
			chrootErr = fmt.Errorf("failed to install '%s':\n%w", dnf5PackageName, chrootErr)
			return chrootErr
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to install '%s' in chroot:\n%w", dnf5PackageName, err)
	}
	return nil
}

// getPackageRepoPathsFromUrl returns a map of canonical package names to download URLs
// for all packages available in the given repoUrl. It uses dnf5 repoquery with a custom
// query format to get both the NEVRA and download location in a single query.
func getPackageRepoPathsFromUrl(repoUrl string) (namesToURLs map[string]string, err error) {
	const (
		dnf5Tool         = "dnf5"
		randomNameLength = 10
		// Use dnf5's --qf to get canonical NEVRA and download URL together, avoiding any
		// filename mangling (e.g., hash prefixes) that the package server may apply.
		queryFormat = "%{name}-%{version}-%{release}.%{arch} %{location}\n"
	)
	var queryCommonArgList = []string{"repoquery", "-q", "-y", "--available", "--disablerepo=*", "--qf", queryFormat}

	logger.Log.Infof("Getting package data from %s", repoUrl)

	// We want to avoid using the same repo name for each repoUrl, so we generate a random name
	randomName, err := randomization.RandomString(randomNameLength, randomization.LegalCharactersAlphaNum)
	if err != nil {
		err = fmt.Errorf("failed to generate random string:\n%w", err)
		return
	}
	repoPathArg := fmt.Sprintf("--repofrompath=mariner-precache-%s,%s", randomName, repoUrl)
	finalArgList := append(queryCommonArgList, repoPathArg)

	namesToURLs = make(map[string]string)
	onStdout := func(line string) {
		name, url, found := strings.Cut(line, " ")
		if found && name != "" && strings.Contains(url, "://") {
			namesToURLs[name] = url
		}
	}

	// Run the dnf5 repoquery command
	err = shell.NewExecBuilder(dnf5Tool, finalArgList...).
		WarnLogLines(shell.DefaultWarnLogLines).
		LogLevel(logrus.TraceLevel, logrus.WarnLevel).
		StdoutCallback(onStdout).
		Execute()
	if err != nil {
		err = fmt.Errorf("failed to run dnf5 repoquery command:\n%w", err)
		return
	}

	return
}

// getPackageRepoUrlsFromRepoFiles returns a map of canonical package names to download URLs
// for all packages available in all RPM repos listed in the system's .repo files.
func getPackageRepoUrlsFromRepoFiles() (namesToURLs map[string]string, err error) {
	const (
		dnf5Tool = "dnf5"
		// Use dnf5's --qf to get canonical NEVRA and download URL together, avoiding any
		// filename mangling (e.g., hash prefixes) that the package server may apply.
		queryFormat = "%{name}-%{version}-%{release}.%{arch} %{location}\n"
	)
	// We have removed all other repo files from the chroot, so we can blindly enable all repos to get the full list of packages
	var queryCommonArgList = []string{"repoquery", "-q", "-y", "--available", "--enablerepo=*", "--qf", queryFormat}

	logger.Log.Info("Getting package data from repo files")

	namesToURLs = make(map[string]string)
	onStdout := func(line string) {
		name, url, found := strings.Cut(line, " ")
		if found && name != "" && strings.Contains(url, "://") {
			namesToURLs[name] = url
		}
	}

	// Run the dnf5 repoquery command
	err = shell.NewExecBuilder(dnf5Tool, queryCommonArgList...).
		WarnLogLines(shell.DefaultWarnLogLines).
		LogLevel(logrus.TraceLevel, logrus.WarnLevel).
		StdoutCallback(onStdout).
		Execute()
	if err != nil {
		err = fmt.Errorf("failed to run dnf5 repoquery command:\n%w", err)
		return
	}

	return
}
