// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// A tool for generating snapshots of built RPMs from local specs.

package codesearch

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/installutils"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/retry"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/rpm"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/safemount"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/shell"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/sliceutils"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/simpletoolchroot"
)

// The codesearch package searches SRPMs for a given regex. It does this by installing the SRPMs in a chroot, then
// searching the sources for the regex via grep. The search is done in parallel, with a semaphore to limit the number of
// parallel jobs. The search is done in the following steps:
// 1. New() is called to create a new CodeSearch object. This initializes the chroot and installs the common packages needed
//    for many %prep sections.
// 2. SearchCode() is called to search for the regex in the SRPMs in the specified directory. This function:
//    a. Finds all the available SRPMs in the directory.
//    b. Queues jobs for each SRPM (throttled by the semaphore):
//       i. Installs the SRPM in the chroot to a custom topdir
//       ii. Installs any extra packages needed for the current SRPM to run its %prep section (as defined in extraPackages list)
//       iii. Runs rpmbuild -bp to prepare the sources
//       iv. Searches the custom topdir using the regex via grep
//       v. Cleans up the custom topdir
//    c. Waits for all the jobs to finish, and sets the results.
// 3. PrintResults() is called to print the search results from the last search.

const (
	chrootName      = "codesearch_chroot"
	numParallelJobs = 10
)

var (
	installMutex = make(chan struct{}, 1)               // Ensure we don't try to install twice at once, this breaks rpm/tdnf
	jobSemaphore = make(chan struct{}, numParallelJobs) // Limit the number of parallel jobs
)

type PackageFixup struct {
	Dependencies     []string          // Extra packages needed for the %prep section
	HoldInstallLock  bool              // Hold the install lock for this package
	ExtraDefinitions map[string]string // Extra rpm defines for this package
}

var (
	// Some packages are just too troublesome to search
	dontSearchList = []string{
		"kernel",     // Kernel config checker gets angry
		"libguestfs", // libguestfs is... special. It tries to run `tdnf install`, and needs all the BRs to work
	}

	// Common packages needed for many %prep sections
	commonDependencies = []string{
		"rubygems-devel",
		"git",
		"javapackages-local-bootstrap",
	}

	// Some packages need extra help to run %prep
	packageFixups = map[string]PackageFixup{
		"ant-junit": {
			Dependencies: []string{"javapackages-tools", "junit"},
		},
		"atinject": {
			Dependencies: []string{"junit"},
		},
		"heimdal": {
			Dependencies: []string{"perl-JSON"},
		},
		"javapackages-tools": {
			ExtraDefinitions: map[string]string{"mvn_artifact": "%nil", "mvn_install": "%nil"},
		},
		"jflex": {
			Dependencies: []string{"ant", "jflex-bootstrap"},
		},
		"jna": {
			Dependencies: []string{"dos2unix", "junit", "objectweb-asm", "ant-junit"},
		},
		"jsr-305": {
			Dependencies: []string{"dos2unix"},
		},
		"jq": {
			Dependencies: []string{"junit"},
		},
		"perl-App-cpanminus": {
			Dependencies: []string{"perl-Pod-Parser", "perl(CPAN::Meta::Requirements)", "perl(version)", "perl-String-ShellQuote"},
		},
		"perl-Archive-Zip": {
			Dependencies: []string{"perl(ExtUtils::MakeMaker)"},
		},
		"perl-DBI": {
			Dependencies: []string{"perl(ExtUtils::MakeMaker)"},
		},
		"prebuilt-ca-certificates": {
			HoldInstallLock: true,
		},
		"prebuilt-ca-certificates-base": {
			HoldInstallLock: true,
		},
		"swig": {
			Dependencies: []string{"python3-pip"},
		},
		"xmvn": {
			Dependencies: []string{"maven"},
		},
	}
)

// CodeSearch is a tool for searching SRPMs for a given regex.
type CodeSearch struct {
	simpleToolChroot          simpletoolchroot.SimpleToolChroot // The chroot to install the SRPMs in and run the search from
	allreadyInstalledPackages map[string]bool                   // A map of all the packages that have been installed to avoid installing them multiple times
	outputStream              io.Writer                         // Optional stream to write output to, if nil, output will be written to stdout
	tmpfsMount                *safemount.Mount                  // Optional tmpfs mount for the chroot
	results                   []SrpmSearchResult                // The results of the last search
}

// SrpmSearchResult contains the results of searching a SRPM.
type SrpmSearchResult struct {
	srpmPath string              // The path to the SRPM
	skipped  bool                // Whether the SRPM was skipped
	matches  map[string][]string // The matches found in the SRPM. The key is the file path, and the value is a list of "line:match" strings
	err      error               // The error that occurred during the search
}

// New creates a new snapshot generator. If the chroot is created successfully, the caller is responsible for calling CleanUp().
// - buildDirPath: The path to create the chroot inside
// - workerTarPath: The path to the worker tarball
// - srpmDirPath: The path to the directory containing the SRPMs
// - outStream: The stream to write output to, if nil, output will be written to stdout
// - useTmpfs: Use a tmpfs mount for the chroot, this provides a huge speedup but uses more memory
func New(buildDirPath, workerTarPath, srpmDirPath string, outStream io.Writer, useTmpfs bool) (newCodeSearch *CodeSearch, err error) {
	newCodeSearch = &CodeSearch{}

	logger.Log.Infof("Creating search chroot in %s", buildDirPath)
	if useTmpfs {
		logger.Log.Infof("Creating tmpfs mount for chroot")
		// Create a tmpfs mount for the chroot, this provides a huge speedup
		newCodeSearch.tmpfsMount, err = safemount.NewMount("", buildDirPath, "tmpfs", 0, "", true)
		if err != nil {
			err = fmt.Errorf("failed to create tmpfs mount. Error:\n%w", err)
			return
		}
	} else {
		logger.Log.Infof("Not using tmpfs mount for chroot")
		newCodeSearch.tmpfsMount = nil
	}

	err = newCodeSearch.simpleToolChroot.InitializeChroot(buildDirPath, chrootName, workerTarPath, srpmDirPath)
	if err != nil {
		// Clean up the tmpfs mount if it was created
		newCodeSearch.CleanUp()
		err = fmt.Errorf("failed to initialize chroot. Error:\n%w", err)
		return
	}

	logger.Log.Infof("Enabling network in chroot")
	err = newCodeSearch.simpleToolChroot.EnableNetwork()
	if err != nil {
		// Clean up the chroot and tmpfs mount if they were created
		newCodeSearch.CleanUp()
		err = fmt.Errorf("failed to enable network. Error:\n%w", err)
		return
	}

	if outStream != nil {
		logger.Log.Infof("Writing output to designated stream")
		newCodeSearch.outputStream = outStream
	}

	return newCodeSearch, err
}

// CleanUp tears down the chroot
// If the chroot was created with a tmpfs mount, it will be cleaned up. The output stream however will not be closed as
// it is not owned by the CodeSearch object.
// - Returns: An error if the chroot could not be cleaned up
func (s *CodeSearch) CleanUp() error {
	var err error
	if s.simpleToolChroot != (simpletoolchroot.SimpleToolChroot{}) {
		err = s.simpleToolChroot.CleanUp()
		if err != nil {
			return fmt.Errorf("failed to cleanup chroot. Error:\n%w", err)
		}
	}
	if s.tmpfsMount != nil {
		s.tmpfsMount.Close()
	}

	// Reset the struct
	*s = CodeSearch{}

	return nil
}

// GenerateSnapshot generates a snapshot of all packages built from the specs inside the input directory.
func (s *CodeSearch) SearchCode(regex, distTag string, packageSearchSet map[string]bool) error {
	if s.simpleToolChroot == (simpletoolchroot.SimpleToolChroot{}) {
		return fmt.Errorf("chroot has not been initialized")
	}

	s.results = []SrpmSearchResult{}

	err := s.simpleToolChroot.RunInChroot(func() (searchErr error) {
		s.results, searchErr = s.runSearchInChroot(regex, distTag, packageSearchSet)
		return searchErr
	})
	if err != nil {
		return fmt.Errorf("failed to search code. Error:\n%w", err)
	}

	return nil
}

// InstallMacros installs the macros needed for the search.
func (s *CodeSearch) installPackages(packagesToInstall []string, cancel chan struct{}) error {
	const rootDir = "/"
	if len(packagesToInstall) == 0 {
		return nil
	}

	// Ensure we don't try to install twice at once, this breaks rpm/tdnf
	if cancel != nil {
		select {
		case installMutex <- struct{}{}:
		case <-cancel:
			return nil
		}
	} else {
		installMutex <- struct{}{}
	}
	defer func() {
		<-installMutex
	}()

	logger.Log.Infof("Installing packages: %v", packagesToInstall)
	for _, repoPackage := range packagesToInstall {
		if s.allreadyInstalledPackages[repoPackage] {
			logger.Log.Debugf("Skipping %s, already installed", repoPackage)
			continue
		} else {
			s.allreadyInstalledPackages[repoPackage] = true
		}
		_, err := installutils.TdnfInstall(repoPackage, rootDir)

		if err != nil {
			err = fmt.Errorf("failed to install package. Error:\n%w", err)
			return err
		}
	}
	return nil
}

func (s *CodeSearch) runSearchInChroot(regex, distTag string, packageSearchSet map[string]bool) (results []SrpmSearchResult, err error) {
	const searchReportIntervalPercent = 10

	logger.Log.Infof("Searching for srpms in %s", s.simpleToolChroot.ChrootRelativeMountDir())

	// For each SRPM in the mount directory, install it and search it.
	srpmsToSearchPaths, err := s.findSrpmPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to walk srpm directory. Error:\n%w", err)
	}

	if len(srpmsToSearchPaths) == 0 {
		logger.Log.Warnf("No srpms found in %s", s.simpleToolChroot.ChrootRelativeMountDir())
		return nil, nil
	}

	// Doing a search, install common packages.
	logger.Log.Infof("Adding common packages needed for many %%prep sections")
	err = s.installPackages(commonDependencies, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to install macro packages. Error:\n%w", err)
	}

	// Search each srpm in parallel
	resultsChannel := make(chan SrpmSearchResult, len(srpmsToSearchPaths))
	cancel := make(chan struct{})
	s.queueWorkers(srpmsToSearchPaths, packageSearchSet, regex, distTag, resultsChannel, cancel)

	// Wait for all the workers to finish, updating the progress as results come in
	numProcessed := 0
	lastReportPercent := 0
	for range srpmsToSearchPaths {
		result := <-resultsChannel
		if result.err != nil {
			// Signal the workers to stop if there is an error
			err = fmt.Errorf("failed to search srpm. Error:\n%w", result.err)
			close(cancel)
			return
		}
		if !result.skipped {
			numProcessed++
			percentProcessed := (numProcessed * 100) / len(srpmsToSearchPaths)
			if percentProcessed-lastReportPercent >= searchReportIntervalPercent {
				logger.Log.Infof("Processed %d/%d srpms (%d%%)", numProcessed, len(srpmsToSearchPaths), percentProcessed)
				lastReportPercent = percentProcessed
			}
		}
		results = append(results, result)
	}
	return
}

func (s *CodeSearch) findSrpmPaths() (foundSrpmPaths []string, err error) {
	const srpmExtention = ".src.rpm"
	err = filepath.Walk(s.simpleToolChroot.ChrootRelativeMountDir(), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, srpmExtention) {
			return nil
		}

		foundSrpmPaths = append(foundSrpmPaths, path)
		return nil
	})
	if err != nil {
		err = fmt.Errorf("failed to walk directory. Error:\n%w", err)
		return nil, err
	}
	return foundSrpmPaths, nil
}

func (s *CodeSearch) queueWorkers(srpmsToSearchPaths []string, packageSearchSet map[string]bool, regex, distTag string, resultsChannel chan SrpmSearchResult, cancel chan struct{}) {
	for _, srpmPath := range srpmsToSearchPaths {
		// skip anything that is alphabetically before "heimdal"
		name, _ := rpm.ExtractNameFromRPMPath(srpmPath)
		skip := sliceutils.Contains(dontSearchList, name, sliceutils.StringMatch)
		if packageSearchSet != nil {
			skip = !packageSearchSet[name]
		}
		if skip {
			logger.Log.Debugf("Skipping %s", srpmPath)
			resultsChannel <- SrpmSearchResult{srpmPath: srpmPath, skipped: true}
			continue
		}
		go func(srpmPath string) {
			// Install the srpm
			select {
			case jobSemaphore <- struct{}{}:
			case <-cancel:
				return
			}
			defer func() {
				<-jobSemaphore
			}()

			topDir, err := s.installSrpmAndDeps(srpmPath, distTag, cancel)
			if err != nil {
				logger.Log.Errorf("Worker failed with error: %v", err)
				resultsChannel <- SrpmSearchResult{err: err}
				return
			}
			defer s.cleanupSrpm(topDir)

			// Allow us to cancel here before running the search
			select {
			case <-cancel:
				return
			default:
			}
			searchResult, err := s.searchSrpm(srpmPath, topDir, regex)
			if err != nil {
				logger.Log.Errorf("Worker failed with error: %v", err)
				resultsChannel <- SrpmSearchResult{err: err}
				return
			}

			// Send the result
			resultsChannel <- searchResult
		}(srpmPath)
	}
}

func (s *CodeSearch) installSrpmAndDeps(srpmPath, distTag string, cancel chan struct{}) (installedDir string, err error) {
	customTopDir := filepath.Join("/tmp", "rpmsearch", srpmPath)
	defines := rpm.DefaultDistroDefines(true, distTag)
	defines["_topdir"] = customTopDir

	logger.Log.Debugf("Preparing (%s) for search", filepath.Base(srpmPath))

	// Some specs are difficult, need to add extra defines to avoid installing all the build requires
	name, err := rpm.ExtractNameFromRPMPath(srpmPath)
	if err != nil {
		err = fmt.Errorf("failed to extract name from rpm path. Error:\n%w", err)
		return "", err
	}
	for def, val := range packageFixups[name].ExtraDefinitions {
		defines[def] = val
	}

	// Some specs need extra packages to be installed
	if len(packageFixups[name].Dependencies) > 0 {
		err = s.installPackages(packageFixups[name].Dependencies, cancel)
		if err != nil {
			err = fmt.Errorf("failed to install extra packages. Error:\n%w", err)
			return "", err
		}
	}

	buildArch, err := rpm.GetRpmArch(runtime.GOARCH)
	if err != nil {
		err = fmt.Errorf("failed to get rpm arch. Error:\n%w", err)
		return "", err
	}

	retry.Run(func() error {
		return rpm.InstallRPM(srpmPath, buildArch, defines)
	}, 3, time.Second*5)
	if err != nil {
		err = fmt.Errorf("failed to install srpm. Error:\n%w", err)
		return "", err
	}

	// Find the spec we just installed
	files, err := rpm.QueryPackage(srpmPath, "[%{FILENAMES}\n]", defines)
	if err != nil {
		err = fmt.Errorf("failed to query package. Error:\n%w", err)
		return "", err
	}
	// Check we have only one *.spec, and grab the name.
	specs := []string{}
	for _, file := range files {
		if strings.HasSuffix(file, ".spec") {
			specs = append(specs, file)
		}
	}
	if len(specs) != 1 {
		err = fmt.Errorf("expected 1 spec file, found %d", len(specs))
		return "", err
	}
	specPath := filepath.Join(customTopDir, "SPECS", specs[0])

	// Some packages get angry if we are doing a `tdnf` install during build, grab the lock
	if packageFixups[name].HoldInstallLock {
		logger.Log.Debugf("Holding install lock for %s", name)
		select {
		case installMutex <- struct{}{}:
		case <-cancel:
			return "", nil
		}
		defer func() {
			logger.Log.Debugf("Releasing install lock for %s", name)
			<-installMutex
		}()
	}
	err = rpm.PrepSourcesFromSPEC(specPath, buildArch, defines)
	if err != nil {
		err = fmt.Errorf("failed to prep sources from srpm (%s). Error:\n%w", srpmPath, err)
		return "", err
	}

	return customTopDir, nil
}

func (s *CodeSearch) cleanupSrpm(topDir string) error {
	err := os.RemoveAll(topDir)
	if err != nil {
		err = fmt.Errorf("failed to remove topdir. Error:\n%w", err)
		return err
	}
	return nil
}

func (s *CodeSearch) searchSrpm(srpmPath, topDir, regex string) (result SrpmSearchResult, err error) {
	logger.Log.Debugf("Searching (%s)", filepath.Base(srpmPath))
	grepOutput, stderr, err := shell.ExecuteInDirectory(topDir, "grep", "-rinP", regex, ".")
	if err != nil {
		// If grep returns a non-zero exit code, it could be because there were no matches.
		// Grep will return error code 1 if no matches are found, but we need to extract it from the os.ExitError.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				// No matches
				logger.Log.Debugf("grep error code 1, no matches found in (%s)", srpmPath)
				return SrpmSearchResult{srpmPath: srpmPath, matches: map[string][]string{}}, nil
			}
		}

		err = fmt.Errorf("failed to grep sources in (%s):\n%v\n%w", topDir, stderr, err)
		return SrpmSearchResult{}, err
	}

	// For each line in the grep output, parse the file and line number. It will be of the
	// form "path/to/file:line:match"
	matches := map[string][]string{}
	for _, line := range strings.Split(grepOutput, "\n") {
		if line == "" {
			continue
		}
		// We only care about the first two colons, the rest is the match.
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			err = fmt.Errorf("unexpected grep output: %s", line)
			return SrpmSearchResult{}, err
		}

		file := parts[0]
		line := parts[1]
		match := parts[2]
		matches[file] = append(matches[file], fmt.Sprintf("%s:%s", line, match))
	}

	logger.Log.Infof(color.GreenString("Found %d matches in %s", len(matches), srpmPath))
	return SrpmSearchResult{srpmPath: srpmPath, matches: matches}, nil
}

func resultPrintln(s string, optionalStream io.Writer) {
	fmt.Println(s)
	if optionalStream != nil {
		fmt.Fprintln(optionalStream, s)
	}
}

// Sort the results by srpm name, then file path.
func (s *CodeSearch) PrintResults() {
	// Sort the resuilts list by srpm name.
	// We want to print the results in a deterministic order.
	sort.Slice(s.results, func(i, j int) bool {
		return s.results[i].srpmPath < s.results[j].srpmPath
	})

	searched := 0
	matchCount := 0

	resultPrintln("Search Results:", s.outputStream)
	for _, result := range s.results {
		if result.skipped {
			continue
		}
		searched++

		// Only print if there are matches
		if len(result.matches) == 0 {
			continue
		}

		matchCount++
		// Print "<SRPM/PATH>:"
		resultPrintln(fmt.Sprintf("\tSRPM: %s", result.srpmPath), s.outputStream)

		// Sort files
		fileList := sliceutils.MapToSlice(result.matches)
		sort.Strings(fileList)
		for _, file := range fileList {
			matches := result.matches[file]
			// Sort by line number
			sort.Strings(matches)
			for _, match := range matches {
				// Print "  <FILE>: <LINE>:     <MATCH>"
				resultPrintln(fmt.Sprintf("\t\t%s:\t\t%s", file, match), s.outputStream)
			}
		}
	}

	baseString := fmt.Sprintf("Searched (%d) srpms, found matches in (%d) of them", searched, matchCount)
	if matchCount > 0 {
		logger.Log.Infof(color.GreenString(baseString))
	} else {
		logger.Log.Infof(color.RedString(baseString))
	}
}
