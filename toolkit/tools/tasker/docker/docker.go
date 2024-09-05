// // Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package docker

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/directory"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/pkgjson"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/randomization"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/resources"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/tasker/buildconfig"
	"github.com/sirupsen/logrus"
)

const (
	baseImage     = "mcr.microsoft.com/azurelinux/local_builder/base"
	SrpmImageTag  = "mcr.microsoft.com/azurelinux/local_builder/srpm"
	RpmImageTag   = "mcr.microsoft.com/azurelinux/local_builder/rpm"
	CacheImageTag = "mcr.microsoft.com/azurelinux/local_builder/cache"
	ImageBase     = baseImage
)

type Docker struct {
	didBuild  bool
	inputFile string
	mutex     *sync.Mutex
}

type PrepScript int

const (
	None PrepScript = iota
	CreateReposAndRun
)

var (
	dockerState = map[string]*Docker{
		ImageBase: {
			didBuild:  false,
			inputFile: resources.AssetsBaseDockerFile,
			mutex:     &sync.Mutex{},
		},
		SrpmImageTag: {
			didBuild:  false,
			inputFile: resources.AssetsSrpmDockerFile,
			mutex:     &sync.Mutex{},
		},
		RpmImageTag: {
			didBuild:  false,
			inputFile: resources.AssetsRpmDockerFile,
			mutex:     &sync.Mutex{},
		},
		CacheImageTag: {
			didBuild:  false,
			inputFile: resources.AssetsCacheDockerFile,
			mutex:     &sync.Mutex{},
		},
	}

	prepScripts = map[PrepScript]string{
		None:              "",
		CreateReposAndRun: "create_repos_and_run.sh",
	}
)

type DockerOverlay struct {
	Source   string
	Dest     string
	Priority int
}

type DockerMount struct {
	Source string
	Dest   string
}

// const dockerLogLevel = logrus.InfoLevel
const dockerLogLevel = logrus.DebugLevel

// Run a command and commit the container to an image with the given tag. Don't use a dockerfile since this is so simple
func buildImage(tag string) error {
	if _, ok := dockerState[tag]; !ok {
		return fmt.Errorf("unknown tag: %s", tag)
	}

	// Create the base image if it doesn't exist
	if tag != ImageBase {
		if err := buildImage(ImageBase); err != nil {
			return err
		}
	}

	dockerState[tag].mutex.Lock()
	defer dockerState[tag].mutex.Unlock()

	if dockerState[tag].didBuild {
		return nil
	}

	tempWorkDir, err := os.MkdirTemp("", "dockerbuilder")
	if err != nil {
		return fmt.Errorf("error creating temp work dir: %s", err)
	}
	defer os.RemoveAll(tempWorkDir)

	for _, assetFile := range resources.DockerAssets {
		dstPath := path.Join(tempWorkDir, path.Base(assetFile))
		err = file.CopyResourceFile(resources.ResourcesFS, assetFile, dstPath, os.ModePerm, os.ModePerm)
		if err != nil {
			return fmt.Errorf("error copying dockerfile: %s", err)
		}
	}
	// Copy the dockerfile to the temp dir
	dockerDstPath := path.Join(tempWorkDir, "Dockerfile")
	err = file.CopyResourceFile(resources.ResourcesFS, dockerState[tag].inputFile, dockerDstPath, os.ModePerm, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error copying dockerfile: %s", err)
	}

	// Build the image
	randomCacheBustVal := ""
	if tag == ImageBase {
		randomCacheBustVal, _ = randomization.RandomString(24, randomization.LegalCharactersHex)
	}
	_, stderr, err := shell.NewExecBuilder("docker", "build", "--build-arg", "CACHEBUST="+randomCacheBustVal, "--network", "host", "-t", tag, "-f", dockerDstPath, ".").WorkingDirectory(tempWorkDir).LogLevel(logrus.InfoLevel, logrus.InfoLevel).ExecuteCaptureOuput()
	if err != nil {
		return fmt.Errorf("error building image: %s", stderr)
	}

	dockerState[tag].didBuild = true
	return nil
}

// Run command in container. Mount points are passed as arguments.
func Run(command string, args []string, outputMountPoint *DockerMount, overlayMounts []DockerOverlay, deps []*pkgjson.PackageVer, tag string, prepScript PrepScript, logFilePath string, printDebug bool) (stdout, stderr string, err error) {
	if err := buildImage(tag); err != nil {
		return "", "", err
	}

	mountPointArg := []string{}
	if outputMountPoint != nil {
		mountPointArg = append(mountPointArg, "-v", fmt.Sprintf("%s:%s", outputMountPoint.Source, outputMountPoint.Dest))
	}

	baseWorkDir, err := os.MkdirTemp(buildconfig.CurrentBuildConfig.TempDir, "docker-overlay")
	// We do not defer removal right away, only clean up on success so we can debug
	if err != nil {
		return "", "", fmt.Errorf("error creating temp work dir: %s", err)
	}

	for _, overlay := range overlayMounts {
		// docker run -it --rm \
		// --mount 'type=volume,dst=/repos,volume-driver=local,,volume-opt=device=overlay,"volume-opt=o=lowerdir=/home/damcilva/repos/CBL-Mariner/toolkit/tools/newsched_rpm/build/RPMS/,upperdir=/tmp/tmp.pqibMdHg7u/upper,workdir=/tmp/tmp.pqibMdHg7u/work"' \
		// "mcr.microsoft.com/azurelinux/local_builder/srpm" \
		// bash
		src := overlay.Source
		err = directory.EnsureDirExists(src)
		if err != nil {
			return "", "", fmt.Errorf("error creating overlay mount: %s", err)
		}
		dst := overlay.Dest

		overlayDir, err := os.MkdirTemp(baseWorkDir, "overlay")
		if err != nil {
			return "", "", fmt.Errorf("error creating temp work dir: %s", err)
		}

		workDir := path.Join(overlayDir, "work")
		err = directory.EnsureDirExists(workDir)
		if err != nil {
			return "", "", fmt.Errorf("error creating overlay mount: %s", err)
		}
		upperDir := path.Join(overlayDir, "upper")
		err = directory.EnsureDirExists(upperDir)
		if err != nil {
			return "", "", fmt.Errorf("error creating overlay mount: %s", err)
		}
		basicArgs := fmt.Sprintf("type=volume,dst=%s,volume-driver=local,volume-opt=type=overlay,volume-opt=device=overlay", dst)
		overlayArgs := fmt.Sprintf(`"volume-opt=o=lowerdir=%s,upperdir=%s,workdir=%s"`, src, upperDir, workDir)
		mountPointArg = append(mountPointArg, "--mount", fmt.Sprintf("%s,%s", basicArgs, overlayArgs))
	}

	// Add the users to the container
	setupScriptArgs := []string{}
	switch prepScript {
	case CreateReposAndRun:
		setupScriptArgs = []string{prepScripts[prepScript]}
		if printDebug {
			setupScriptArgs = append(setupScriptArgs, "--print-to-stderr")
		}

		if outputMountPoint != nil {
			localUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
			setupScriptArgs = append(setupScriptArgs, fmt.Sprintf("--user=%s", localUser), fmt.Sprintf("--path=%s", outputMountPoint.Dest))
		}
		mountPointArg = append(mountPointArg, "-v", "/etc/passwd:/etc/passwd:ro", "-v", "/etc/group:/etc/group:ro", "-v", "/etc/shadow:/etc/shadow:ro")

		for _, overlay := range overlayMounts {
			if overlay.Dest == "/repos/upstream" {
				setupScriptArgs = append(setupScriptArgs, fmt.Sprintf("--upstream-repo-priority=%d", overlay.Priority))
			} else {
				//setupScriptArgs = append(setupScriptArgs, fmt.Sprintf("--repodir=%s:%d", overlayMounts[i+1], i+1))
				setupScriptArgs = append(setupScriptArgs, fmt.Sprintf("--repodir=%s:%d", overlay.Dest, overlay.Priority))
			}
		}

		// Add the deps
		for _, dep := range deps {
			setupScriptArgs = append(setupScriptArgs, fmt.Sprintf("--install-dep=%s", dep.Name))
		}
	}

	dockerArgs := []string{"run", "--rm", "--network", "host"}
	dockerArgs = append(dockerArgs, mountPointArg...)
	dockerArgs = append(dockerArgs, tag)
	dockerArgs = append(dockerArgs, setupScriptArgs...)
	dockerArgs = append(dockerArgs, command)
	dockerArgs = append(dockerArgs, args...)

	//output, stderr, err := shell.Execute("docker", dockerArgs...)

	docker := shell.NewExecBuilder("docker", dockerArgs...).LogLevel(dockerLogLevel, dockerLogLevel)

	// If log file, use Callbacks() to log each line to the log file
	if logFilePath == "" {
		workdir := filepath.Join(buildconfig.CurrentBuildConfig.TempDir, "dockerlogs")
		err = directory.EnsureDirExists(workdir)
		if err != nil {
			return "", "", fmt.Errorf("error ensuring dir %s exists for log file: %s", workdir, err)
		}
		// Just grab the name and then close the file, we will open again later
		logFileHandle, err := os.CreateTemp(workdir, "run*.log")
		if err != nil {
			return "", "", fmt.Errorf("error creating temp log file: %s", err)
		}
		logFilePath = logFileHandle.Name()
		logFileHandle.Close()
	}

	err = directory.EnsureDirExists(filepath.Dir(logFilePath))
	if err != nil {
		return "", "", fmt.Errorf("error ensuring dir exists for log file: %s", err)
	}
	// Open the file, clearing it if it exists
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return "", "", fmt.Errorf("error opening log file: %s", err)
	}
	defer logFile.Close()
	// Write the command we are running to the log file
	logFile.WriteString(fmt.Sprintf("Running command: docker %s\n", strings.Join(dockerArgs, "' '")+"'"))
	callbackFunc := func(line string) {
		logFile.WriteString(line + "\n")
	}
	docker = docker.Callbacks(callbackFunc, callbackFunc)

	fmt.Printf("docker '%s'\nLogging to %s\n", strings.Join(dockerArgs, "' '"), logFilePath)

	stdout, stderr, err = docker.ExecuteCaptureOuput()
	if err == nil {
		defer os.RemoveAll(baseWorkDir)
	} else {
		logger.Log.Errorf("Docker command failed: %s", err)
		logger.Log.Errorf("    Log file: %s", logFilePath)
		logger.Log.Errorf("    Working directory: %s", baseWorkDir)
	}
	return stdout, stderr, err
}

// Generate standard mounts
func MountsForDirtLevel(allowableDirtLevel int, alwaysCache bool) ([]DockerOverlay, error) {
	repoMounts := []DockerOverlay{}
	for i := 0; i <= allowableDirtLevel && i < buildconfig.CurrentBuildConfig.MaxDirt; i++ {
		err := directory.EnsureDirExists(buildconfig.CurrentBuildConfig.RpmsDirsByDirtLevel[i])
		if err != nil {
			return nil, fmt.Errorf("error ensuring dir exists while creating mounts: %s", err)
		}
		newOverlay := DockerOverlay{
			Source:   buildconfig.CurrentBuildConfig.RpmsDirsByDirtLevel[i],
			Dest:     filepath.Join("/repos", fmt.Sprintf("%d", i)),
			Priority: i,
		}
		repoMounts = append(repoMounts, newOverlay)
	}

	if allowableDirtLevel >= buildconfig.CurrentBuildConfig.MaxDirt {
		repoMounts = append(repoMounts, MountForInput()...)
	}

	if alwaysCache || allowableDirtLevel >= buildconfig.CurrentBuildConfig.MaxDirt {
		repoMounts = append(repoMounts, MountForUpstreamCache()...)
	}

	return repoMounts, nil
}

func MountForInput() []DockerOverlay {
	repoMounts := []DockerOverlay{
		{
			Source:   buildconfig.CurrentBuildConfig.InputRepoDir,
			Dest:     filepath.Join("/repos/", fmt.Sprintf("%d", buildconfig.CurrentBuildConfig.MaxDirt)),
			Priority: buildconfig.CurrentBuildConfig.MaxDirt,
		},
	}
	return repoMounts
}

func MountForUpstreamCache() []DockerOverlay {
	repoMounts := []DockerOverlay{
		{
			Source:   buildconfig.CurrentBuildConfig.RpmsCacheDir,
			Dest:     "/repos/upstream",
			Priority: buildconfig.CurrentBuildConfig.MaxDirt + 1,
		},
	}
	return repoMounts
}

func AllMounts() []DockerOverlay {
	dirtyRepos, err := MountsForDirtLevel(buildconfig.CurrentBuildConfig.MaxDirt, true)
	if err != nil {
		logrus.Fatal(err)
	}
	return dirtyRepos
}
