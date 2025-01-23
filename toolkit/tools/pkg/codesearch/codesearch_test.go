package codesearch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azurelinux/toolkit/tools/internal/file"
	"github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	"github.com/microsoft/azurelinux/toolkit/tools/pkg/simpletoolchroot"
	"github.com/moby/sys/mountinfo"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

var (
	mountPaths = []string{}
	tmpDirRoot string
)

func TestMain(m *testing.M) {
	logger.InitStderrLog()

	workingDir, err := os.Getwd()
	if err != nil {
		logger.Log.Panicf("Failed to get working directory, error: %s", err)
	}

	tmpDirRoot = filepath.Join(workingDir, "_tmp")

	retVal := m.Run()

	// Clean up any mounts we might have created
	err = CleanUp()
	if err != nil {
		fmt.Printf("Failed to clean up mounts. Error:\n%v", err)
		retVal = 1
	} else {
		err = os.RemoveAll(tmpDirRoot)
		if err != nil {
			fmt.Printf("Failed to remove tmp dir %s. Error:\n%v", tmpDirRoot, err)
			retVal = 1
		}
	}

	os.Exit(retVal)
}

// Cleanup, we need to scan all the mounts we might have created and unmount them
func CleanUp() error {
	fmt.Println("Cleaning up mounts...")
	for _, mountPath := range mountPaths {
		fmt.Printf("Checking %s...\n", mountPath)
		isMounted, err := checkIfMounted(mountPath)
		if err != nil {
			return err
		}
		if isMounted {
			fmt.Println("\tUnmounting...")
			err = unix.Unmount(mountPath, unix.MNT_DETACH)
			if err != nil {
				return fmt.Errorf("failed to unmount %s. Error:\n%w", mountPath, err)
			}
		} else {
			fmt.Println("\tClean")
		}
	}
	return nil
}

// createBuildDirForChroot creates a directory to be used as a mount point for a chroot
func createBuildDirForChroot(t *testing.T, name string) (testBuildDir, chrootDir string) {
	t.Helper()
	testBuildDir = filepath.Join(tmpDirRoot, name)
	chrootDir = filepath.Join(testBuildDir, "chroots", chrootName)
	err := os.MkdirAll(chrootDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create mount path %s. Error:\n%v", chrootDir, err)
	}
	mountPaths = append(mountPaths, chrootDir)
	return testBuildDir, chrootDir
}

func checkIfMounted(mountPath string) (bool, error) {
	exists, err := file.PathExists(mountPath)
	if err != nil {
		return false, fmt.Errorf("failed to check if %s exists. Error:\n%w", mountPath, err)
	}
	if !exists {
		return false, nil
	}
	isMounted, err := mountinfo.Mounted(mountPath)
	if err != nil {
		return false, fmt.Errorf("failed to check if %s is mounted. Error:\n%w", mountPath, err)
	}
	return isMounted, nil
}

func TestNewWithBlankChroot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses a chroot")
	}

	buildDirPath, chrootPath := createBuildDirForChroot(t, t.Name())
	workerTarPath := ""
	srpmDirPath := filepath.Join(buildDirPath, "srpm")
	mountPaths = append(mountPaths, srpmDirPath)

	// dummy io.Writer buffer
	outStream := new(bytes.Buffer)
	useTmpfs := true

	// Create srpm dir
	err := os.MkdirAll(srpmDirPath, os.ModePerm)
	assert.NoError(t, err)

	cs, err := New(chrootPath, workerTarPath, srpmDirPath, outStream, useTmpfs)
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	assert.NotNil(t, cs.tmpfsMount)
	assert.NotNil(t, cs.simpleToolChroot)
	assert.NotNil(t, cs.outputStream)

	// Clean up
	err = cs.CleanUp()
	assert.NoError(t, err)
	assert.Nil(t, cs.tmpfsMount)
	assert.Equal(t, simpletoolchroot.SimpleToolChroot{}, cs.simpleToolChroot)
	assert.Nil(t, cs.outputStream)
}

func TestSearchCode(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test must be run as root because it uses a chroot")
	}

	buildDirPath, _ := createBuildDirForChroot(t, t.Name()+"_noChroot")
	srpmDir := filepath.Join(buildDirPath, "srpm")
	mountPaths = append(mountPaths, srpmDir)
	err := os.MkdirAll(srpmDir, os.ModePerm)
	assert.NoError(t, err)

	// Chroot not initialized
	s := &CodeSearch{}
	err = s.SearchCode("regex", "", "distTag", nil)
	assert.EqualError(t, err, "chroot has not been initialized")

	// Chroot initialized
	buildDirPath, chrootPath := createBuildDirForChroot(t, t.Name()+"_yesChroot")
	srpmDir = filepath.Join(buildDirPath, "srpm")
	mountPaths = append(mountPaths, srpmDir)
	err = os.MkdirAll(srpmDir, os.ModePerm)
	assert.NoError(t, err)

	s, err = New(chrootPath, "", srpmDir, nil, false)
	assert.NoError(t, err)
	err = s.SearchCode("regex", "", "distTag", nil)
	assert.NoError(t, err)

	// Chroot cleaned
	err = s.CleanUp()
	assert.NoError(t, err)
	err = s.SearchCode("regex", "", "distTag", nil)
	assert.EqualError(t, err, "chroot has not been initialized")
}

func TestPrintResults(t *testing.T) {
	s := &CodeSearch{
		results: []SrpmSearchResult{
			{
				srpmPath: "srpm1",
				matches: map[string][]string{
					"file1": {"line1", "line2"},
					"file2": {"line3"},
				},
			},
			{
				srpmPath: "srpm2",
				matches: map[string][]string{
					"file3": {"line4"},
				},
			},
			{
				srpmPath: "srpm3",
				matches:  map[string][]string{},
			},
			{
				srpmPath: "srpm4",
				skipped:  true,
			},
		},
		outputStream: os.Stdout, // Replace with your desired output stream
	}

	// Redirect output to a buffer for testing
	buf := new(bytes.Buffer)
	s.outputStream = buf

	s.PrintResults()

	expectedOutput :=
		`Search Results:
	SRPM: srpm1
		file1:		line1
		file1:		line2
		file2:		line3
	SRPM: srpm2
		file3:		line4
`
	assert.Equal(t, expectedOutput, buf.String())
}

// func TestSearchSrpmMissingDir(t *testing.T) {
// 	tempDir := "not_a_real_dir"
// 	s := &CodeSearch{}
// 	result, err := s.searchSrpm("srpm", "", tempDir, "regex")
// 	assert.Error(t, err)
// 	assert.Equal(t, SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{}}, result)
// }

func TestSearchSrpm(t *testing.T) {
	type testCase struct {
		name string
		// Each element in the slice represents line in a file, which is in a directory
		dirsWithFilesWithLines map[string]map[string][]string
		regex                  string
		expectedResult         SrpmSearchResult
	}

	testCases := []testCase{
		{
			name:                   "No files",
			dirsWithFilesWithLines: map[string]map[string][]string{},
			regex:                  "regex",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{}},
		},
		{
			name:                   "No matches",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"wrong_string"}}},
			regex:                  "match",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{}},
		},
		{
			name:                   "Single match",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match1"}}},
			regex:                  "match1",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./file0": {"1:match1"}}},
		},
		{
			name:                   "Multiple matches",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match2", "match3"}}},
			regex:                  "match[0-9]",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./file0": {"1:match2", "2:match3"}}},
		},
		{
			name:                   "Multiple files",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match2", "match3"}, "file1": {"match4"}}},
			regex:                  "match[0-9]",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./file0": {"1:match2", "2:match3"}, "./file1": {"1:match4"}}, skipped: false},
		},
		{
			name:                   "Multiple directories",
			dirsWithFilesWithLines: map[string]map[string][]string{"dir0": {"file0": {"match2", "match3"}}, "dir1": {"file1": {"match4"}}},
			regex:                  "match[0-9]",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./dir0/file0": {"1:match2", "2:match3"}, "./dir1/file1": {"1:match4"}}, skipped: false},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			s := &CodeSearch{}
			for dir, filesWithLines := range tc.dirsWithFilesWithLines {
				if dir != "" {
					dir = filepath.Join(tempDir, dir)
					err := os.MkdirAll(dir, os.ModePerm)
					assert.NoError(t, err)
				} else {
					dir = tempDir
				}
				for filePath, lines := range filesWithLines {
					filePath = filepath.Join(dir, filePath)
					err := file.WriteLines(lines, filePath)
					assert.NoError(t, err)
				}
			}
			result, err := s.searchSrpm("srpm", tempDir, tc.regex, "")
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestSearchSrpmWithFileFilter(t *testing.T) {
	type testCase struct {
		name string
		// Each element in the slice represents line in a file, which is in a directory
		dirsWithFilesWithLines map[string]map[string][]string
		regex                  string
		fileFilter             string
		expectedResult         SrpmSearchResult
		expectedError          bool
	}

	testCases := []testCase{
		{
			name:                   "No files, filter",
			dirsWithFilesWithLines: map[string]map[string][]string{},
			regex:                  "regex",
			fileFilter:             "file0",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{}},
		},
		{
			name:                   "No matches, filter",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"wrong_string"}}},
			regex:                  "match",
			fileFilter:             "file0",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{}},
		},
		{
			name:                   "Single match, filter",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match1"}}},
			regex:                  "match1",
			fileFilter:             "file0",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./file0": {"1:match1"}}},
		},
		{
			name:                   "Multiple matches, filter",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match2", "match3"}}},
			regex:                  "match[0-9]",
			fileFilter:             "file0",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./file0": {"1:match2", "2:match3"}}},
		},
		{
			name:                   "Multiple files, filter",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match2", "match3"}, "file1": {"match4"}}},
			regex:                  "match[0-9]",
			fileFilter:             "file0",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./file0": {"1:match2", "2:match3"}}, skipped: false},
		},
		{
			name:                   "Multiple directories, filter",
			dirsWithFilesWithLines: map[string]map[string][]string{"dir0": {"file0": {"match2", "match3"}}, "dir1": {"file1": {"match4"}}},
			regex:                  "match[0-9]",
			fileFilter:             "file0",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./dir0/file0": {"1:match2", "2:match3"}}, skipped: false},
		},
		{
			name:                   "No filter matches",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match2", "match3"}, "file1": {"match4"}}},
			regex:                  "match[0-9]",
			fileFilter:             "none_of_these_files",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{}},
		},
		{
			name:                   "Filter handles regex",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match1"}}},
			regex:                  "match1",
			fileFilter:             "file[0-9]",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{"./file0": {"1:match1"}}},
		},
		{
			name:                   "Filter handles regex, no matches",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match1"}}},
			regex:                  "match1",
			fileFilter:             ".*not_a_real_file.*",
			expectedResult:         SrpmSearchResult{srpmPath: "srpm", matches: map[string][]string{}},
		},
		{
			name:                   "Invalid regex",
			dirsWithFilesWithLines: map[string]map[string][]string{"": {"file0": {"match1"}}},
			regex:                  "[",
			fileFilter:             "file0",
			expectedError:          true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			s := &CodeSearch{}
			for dir, filesWithLines := range tc.dirsWithFilesWithLines {
				if dir != "" {
					dir = filepath.Join(tempDir, dir)
					err := os.MkdirAll(dir, os.ModePerm)
					assert.NoError(t, err)
				} else {
					dir = tempDir
				}
				for filePath, lines := range filesWithLines {
					filePath = filepath.Join(dir, filePath)
					err := file.WriteLines(lines, filePath)
					assert.NoError(t, err)
				}
			}
			result, err := s.searchSrpm("srpm", tempDir, tc.regex, tc.fileFilter)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}
