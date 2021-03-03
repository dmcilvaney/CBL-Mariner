// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// A worker for building packages locally

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
	"microsoft.com/pkggen/internal/exe"
	"microsoft.com/pkggen/internal/file"
	"microsoft.com/pkggen/internal/logger"
	"microsoft.com/pkggen/internal/packagerepo/repomanager/rpmrepomanager"
	"microsoft.com/pkggen/internal/rpm"
	"microsoft.com/pkggen/internal/safechroot"
	"microsoft.com/pkggen/internal/shell"
	"microsoft.com/pkggen/internal/sliceutils"
)

const (
	chrootRpmBuildRoot      = "/usr/src/mariner"
	chrootLocalRpmsDir      = "/localrpms"
	chrootLocalRpmsCacheDir = "/upstream-cached-rpms"
)

var (
	app                     = kingpin.New("pkgworker", "A worker for building packages locally")
	srpmFile                = exe.InputFlag(app, "Full path to the SRPM to build")
	workDir                 = app.Flag("work-dir", "The directory to create the build folder").Required().String()
	workerTar               = app.Flag("worker-tar", "Full path to worker_chroot.tar.gz").Required().ExistingFile()
	repoFile                = app.Flag("repo-file", "Full path to local.repo").Required().ExistingFile()
	rpmsDirPath             = app.Flag("rpm-dir", "The directory to use as the local repo and to submit RPM packages to").Required().ExistingDir()
	srpmsDirPath            = app.Flag("srpm-dir", "The output directory for source RPM packages").Required().String()
	cacheDir                = app.Flag("cache-dir", "The cache directory containing downloaded dependency RPMS from CBL-Mariner Base").Required().ExistingDir()
	noCleanup               = app.Flag("no-cleanup", "Whether or not to delete the chroot folder after the build is done").Bool()
	distTag                 = app.Flag("dist-tag", "The distribution tag the SPEC will be built with.").Required().String()
	distroReleaseVersion    = app.Flag("distro-release-version", "The distro release version that the SRPM will be built with").Required().String()
	distroBuildNumber       = app.Flag("distro-build-number", "The distro build number that the SRPM will be built with").Required().String()
	rpmmacrosFile           = app.Flag("rpmmacros-file", "Optional file path to an rpmmacros file for rpmbuild to use").ExistingFile()
	runCheck                = app.Flag("run-check", "Run the check during package build").Bool()
	packagesToInstall       = app.Flag("install-package", "Filepaths to RPM packages that should be installed before building.").Strings()
	targetPackagesToInstall = app.Flag("target-install-package", "Filepaths to RPM packages that should be installed into target sysroot before building.").Strings()
	outArch                 = app.Flag("out-arch", "Architecture of resulting package").String()

	logFile  = exe.LogFileFlag(app)
	logLevel = exe.LogLevelFlag(app)
)

var (
	packageUnavailableRegex = regexp.MustCompile(`^No package \\x1b\[1m\\x1b\[30m(.+) \\x1b\[0mavailable`)
)

func main() {
	app.Version(exe.ToolkitVersion)
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logger.InitBestEffort(*logFile, *logLevel)

	rpmsDirAbsPath, err := filepath.Abs(*rpmsDirPath)
	logger.PanicOnError(err, "Unable to find absolute path for RPMs directory '%s'", *rpmsDirPath)

	srpmsDirAbsPath, err := filepath.Abs(*srpmsDirPath)
	logger.PanicOnError(err, "Unable to find absolute path for SRPMs directory '%s'", *srpmsDirPath)

	srpmName := strings.TrimSuffix(filepath.Base(*srpmFile), ".src.rpm")
	chrootDir := filepath.Join(*workDir, srpmName)

	defines := rpm.DefaultDefines()
	defines[rpm.DistTagDefine] = *distTag
	defines[rpm.DistroReleaseVersionDefine] = *distroReleaseVersion
	defines[rpm.DistroBuildNumberDefine] = *distroBuildNumber

	builtRPMs, err := buildSRPMInChroot(chrootDir, rpmsDirAbsPath, *workerTar, *srpmFile, *repoFile, *rpmmacrosFile, *outArch, defines, *noCleanup, *runCheck, *packagesToInstall, *targetPackagesToInstall)
	logger.PanicOnError(err, "Failed to build SRPM '%s'. For details see log file: %s .", *srpmFile, *logFile)

	err = copySRPMToOutput(*srpmFile, srpmsDirAbsPath)
	logger.PanicOnError(err, "Failed to copy SRPM '%s' to output directory '%s'.", *srpmFile, rpmsDirAbsPath)

	// On success write a comma-seperated list of RPMs built to stdout that can be parsed by the invoker.
	// Any output from logger will be on stderr so stdout will only contain this output.
	fmt.Printf(strings.Join(builtRPMs, ","))
}

func copySRPMToOutput(srpmFilePath, srpmOutputDirPath string) (err error) {
	const srpmsDirName = "SRPMS"

	srpmFileName := filepath.Base(srpmFilePath)
	srpmOutputFilePath := filepath.Join(srpmOutputDirPath, srpmFileName)

	err = file.Copy(srpmFilePath, srpmOutputFilePath)

	return
}

func buildSRPMInChroot(chrootDir, rpmDirPath, workerTar, srpmFile, repoFile, rpmmacrosFile, outArch string, defines map[string]string, noCleanup, runCheck bool, packagesToInstall, targetPackagesToInstall []string) (builtRPMs []string, err error) {
	const (
		existingChrootDir = false
		squashErrors      = false

		overlaySource  = ""
		overlayWorkDir = "/overlaywork"
		rpmDirName     = "RPMS"
	)

	srpmBaseName := filepath.Base(srpmFile)
	logger.Log.Infof("Building (%s)", srpmBaseName)

	// Create the chroot used to build the SRPM
	chroot := safechroot.NewChroot(chrootDir, existingChrootDir)

	overlayMount, overlayExtraDirs := safechroot.NewOverlayMountPoint(chroot.RootDir(), overlaySource, chrootLocalRpmsDir, rpmDirPath, chrootLocalRpmsDir, overlayWorkDir)
	rpmCacheMount := safechroot.NewMountPoint(*cacheDir, chrootLocalRpmsCacheDir, "", safechroot.BindMountPointFlags, "")
	mountPoints := []*safechroot.MountPoint{overlayMount, rpmCacheMount}
	extraDirs := append(overlayExtraDirs, chrootLocalRpmsCacheDir)

	err = chroot.Initialize(workerTar, extraDirs, mountPoints)
	if err != nil {
		return
	}
	defer chroot.Close(noCleanup)

	// Place extra files that will be needed to build into the chroot
	srpmFileInChroot, err := copyFilesIntoChroot(chroot, srpmFile, repoFile, rpmmacrosFile)
	if err != nil {
		return
	}

	err = chroot.Run(func() (err error) {
		return buildRPMFromSRPMInChroot(srpmFileInChroot, outArch, runCheck, defines, packagesToInstall, targetPackagesToInstall)
	})

	if err != nil {
		return
	}

	rpmBuildOutputDir := filepath.Join(chroot.RootDir(), chrootRpmBuildRoot, rpmDirName)
	builtRPMs, err = moveBuiltRPMs(rpmBuildOutputDir, rpmDirPath)
	if err != nil {
		return
	}

	logger.Log.Infof("Built (%s) -> %v", srpmBaseName, builtRPMs)

	return
}

func buildRPMFromSRPMInChroot(srpmFile, outArch string, runCheck bool, defines map[string]string, packagesToInstall, targetPackagesToInstall []string) (err error) {
	const (
		sysrootDir = "/opt/cross/aarch64-mariner-linux-gnu/sysroot"
	)

	logger.Log.Debugf("packagesToInstall (%s)", packagesToInstall)
	logger.Log.Debugf("targetPackagesToInstall (%s)", targetPackagesToInstall)

	// Convert /localrpms into a repository that a package manager can use
	err = rpmrepomanager.CreateRepo(chrootLocalRpmsDir)
	if err != nil {
		return
	}

	buildArch, err := rpm.GetRpmArch(runtime.GOARCH)
	if err != nil {
		return
	}

	// install any additional packages, such as build dependencies.
	err = tdnfInstall(packagesToInstall, buildArch, "/")
	if err != nil {
		return
	}

	// Check if we are cross compiling. If so, populate the sysroot
	if (buildArch != outArch) && (outArch != "noarch") {
		// Another hack. Adding a spot for initial sysroot packages. Should try
		// to remove this for the final design.
		initialSysroot := make([]string, 0)
		initialSysroot = append(initialSysroot, "mariner-release")
		initialSysroot = append(initialSysroot, "filesystem")
		err = tdnfInstall(initialSysroot, outArch, sysrootDir)
		if err != nil {
			return
		}
		// This is a bit of a hack. We need a way for the spec file to tell us that
		// it doesn't need certain BuildRequires to be installed into the sysroot.
		// For example, the cross-toolchain really should only be installed into
		// the chroot but not installed into the sysroot
		crossToolchain := make([]string, 0)
		crossToolchain = append(crossToolchain, "aarch64-mariner-linux-gnu-cross-gcc")
		// Hack to make glibc build. Apparently it needs perl(File::Find)
		crossToolchain = append(crossToolchain, "perl(File::Find)")
		err = tdnfInstall(crossToolchain, buildArch, "/")
		if err != nil {
			return
		}
		err = tdnfInstall(targetPackagesToInstall, outArch, sysrootDir)
		if err != nil {
			return
		}
	} else {
		// If we are doing a native build but targetPackagesToInstall is not nil here, we goofed
		if len(targetPackagesToInstall) != 0 {
			err = fmt.Errorf("native build scenario detected but target packages to install is non-zero (%v)", targetPackagesToInstall)
			return
		}
	}

	// Remove all libarchive files on the system before issuing a build.
	// If the build environment has libtool archive files present, gnu configure
	// could detect it and create more libtool archive files which can cause
	// build failures.
	err = removeLibArchivesFromSystem()
	if err != nil {
		return
	}

	// Build the SRPM
	if runCheck {
		err = rpm.BuildRPMFromSRPM(srpmFile, outArch, defines)
	} else {
		err = rpm.BuildRPMFromSRPM(srpmFile, outArch, defines, "--nocheck")
	}

	return
}

func moveBuiltRPMs(rpmOutDir, dstDir string) (builtRPMs []string, err error) {
	const rpmExtension = ".rpm"
	err = filepath.Walk(rpmOutDir, func(path string, info os.FileInfo, fileErr error) (err error) {
		if fileErr != nil {
			return fileErr
		}

		// Only copy regular files (not unix sockets, directories, links, ...)
		if !info.Mode().IsRegular() {
			return
		}

		if !strings.HasSuffix(path, rpmExtension) {
			return
		}

		// Get the relative path of the RPM, this will include the architecture directory it lives in.
		// Then join the relative path to the destination directory, this will ensure the RPM gets placed
		// in its correct architecture directory.
		relPath, err := filepath.Rel(rpmOutDir, path)
		if err != nil {
			return
		}

		dstFile := filepath.Join(dstDir, relPath)
		err = file.Move(path, dstFile)
		if err != nil {
			return
		}

		builtRPMs = append(builtRPMs, dstFile)
		return
	})

	return
}

func tdnfInstall(packages []string, outArch, installRoot string) (err error) {
	const (
		alreadyInstalledPostfix = "is already installed."
		noMatchingPackagesErr   = "Error(1011) : No matching packages"
		packageMatchGroup       = 1
	)

	if len(packages) == 0 {
		return
	}

	// TDNF supports requesting versioned packages in the form of {name}-{version}.{dist}.{arch}.
	// The packages to install list may contain file paths to rpm files so those will need to be filtered:
	// - Strip any .rpm from packages as TDNF does not support requesting a package with the extension.
	// - Strip any filepath from packages.
	for i := range packages {
		packages[i] = filepath.Base(strings.TrimSuffix(packages[i], ".rpm"))
	}

	installArgs := []string{"install", "-y"}
	installArgs = append(installArgs, "--targetarch", outArch)
	// This --refresh can be needed to workaround some oddities with --targetarch.
	installArgs = append(installArgs, "--refresh")
	// Another hack. To install to the sysroot, tdnf requires that releasever be set. Otherwise you get
	// this fun tdnf error:
	// Error(1022) : distroverpkg config entry is set to a package that is not installed. Check /etc/tdnf/tdnf.conf
	// For some reason, tdnf cannot find our standard releasever which is supplied by mariner-release.
	// We might need to out-of-band install mariner-release into the sysroot. Until then, overload
	// the releasever inline in the tdnf command.
	installArgs = append(installArgs, "--releasever", "1.0")
	installArgs = append(installArgs, "--installroot", installRoot)
	installArgs = append(installArgs, packages...)
	stdout, stderr, err := shell.Execute("tdnf", installArgs...)
	foundNoMatchingPackages := false

	if err != nil {
		logger.Log.Warnf("Failed to install build requirements. stderr: %s\nstdout: %s", stderr, stdout)
		// TDNF will output an error if all packages are already installed.
		// Ignore it iff there is no other error present in stderr.
		splitStderr := strings.Split(stderr, "\n")
		for _, line := range splitStderr {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}

			if strings.Contains(trimmedLine, noMatchingPackagesErr) {
				foundNoMatchingPackages = true
			}

			if !strings.HasSuffix(trimmedLine, alreadyInstalledPostfix) && trimmedLine != noMatchingPackagesErr {
				err = fmt.Errorf(trimmedLine)
				return
			}
		}
		err = nil
	}

	// TDNF will ignore unavailable packages that have been requested to be installed without reporting an error code.
	// Search the stdout of TDNF for such a failure and warn the user.
	// This may happen if a SPEC requires the the path to a tool (e.g. /bin/cp), so mark it as a warning for now.
	var failedToInstall []string
	splitStdout := strings.Split(stdout, "\n")
	for _, line := range splitStdout {
		trimmedLine := strings.TrimSpace(line)
		matches := packageUnavailableRegex.FindStringSubmatch(trimmedLine)
		if len(matches) == 0 {
			continue
		}

		failedToInstall = append(failedToInstall, matches[packageMatchGroup])
	}

	// TDNF will output the error "Error(1011) : No matching packages" if all packages could not be found.
	// In this case it will not print any of the individual packages that failed.
	if foundNoMatchingPackages && len(failedToInstall) == 0 {
		failedToInstall = packages
	}

	if len(failedToInstall) != 0 {
		err = fmt.Errorf("unable to install the following packages: %v", failedToInstall)
	}

	return
}

// removeLibArchivesFromSystem removes all libarchive files on the system. If
// the build environment has libtool archive files present, gnu configure could
// detect it and create more libtool archive files which can cause build failures.
func removeLibArchivesFromSystem() (err error) {
	dirsToExclude := []string{"/proc", "/dev", "/sys", "/run"}

	err = filepath.Walk("/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories that are meant for device files and kernel virtual filesystems.
		// These will not contain .la files and are mounted into the safechroot from the host.
		if info.IsDir() && sliceutils.Find(dirsToExclude, path) != -1 {
			return filepath.SkipDir
		}

		if strings.HasSuffix(info.Name(), ".la") {
			return os.Remove(path)
		}

		return nil
	})

	if err != nil {
		logger.Log.Warnf("Unable to remove lib archive file: %s", err)
	}

	return
}

// copyFilesIntoChroot copies several required build specific files into the chroot.
func copyFilesIntoChroot(chroot *safechroot.Chroot, srpmFile, repoFile, rpmmacrosFile string) (srpmFileInChroot string, err error) {
	const (
		chrootRepoDestDir = "/etc/yum.repos.d"
		chrootSrpmDestDir = "/root/SRPMS"
		rpmmacrosDest     = "/usr/lib/rpm/macros.d/macros.override"
	)

	repoFileInChroot := filepath.Join(chrootRepoDestDir, filepath.Base(repoFile))
	srpmFileInChroot = filepath.Join(chrootSrpmDestDir, filepath.Base(srpmFile))

	filesToCopy := []safechroot.FileToCopy{
		safechroot.FileToCopy{
			Src:  repoFile,
			Dest: repoFileInChroot,
		},
		safechroot.FileToCopy{
			Src:  srpmFile,
			Dest: srpmFileInChroot,
		},
	}

	if rpmmacrosFile != "" {
		rpmmacrosCopy := safechroot.FileToCopy{
			Src:  rpmmacrosFile,
			Dest: rpmmacrosDest,
		}
		filesToCopy = append(filesToCopy, rpmmacrosCopy)
	}

	err = chroot.AddFiles(filesToCopy...)
	return
}
