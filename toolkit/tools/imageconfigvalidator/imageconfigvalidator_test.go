// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"microsoft.com/pkggen/imagegen/configuration"
	"microsoft.com/pkggen/internal/logger"
)

func TestMain(m *testing.M) {
	logger.InitStderrLog()
	os.Exit(m.Run())
}

func TestShouldSucceedValidatingDefaultConfigs(t *testing.T) {
	const (
		configDirectory = "../../imageconfigs/"
	)
	checkedConfigs := 0
	configFiles, err := ioutil.ReadDir(configDirectory)
	assert.NoError(t, err)

	for _, file := range configFiles {
		if !file.IsDir() && strings.Contains(file.Name(), ".json") {
			configPath := filepath.Join(configDirectory, file.Name())

			fmt.Println("Validating ", configPath)

			config, err := configuration.Load(configPath)
			assert.NoError(t, err)

			err = ValidateConfiguration(config)
			assert.NoError(t, err)
			checkedConfigs++
		}
	}
	// Make sure we found at least one config to validate
	assert.GreaterOrEqual(t, checkedConfigs, 1)
}

func TestShouldFailEmptyConfig(t *testing.T) {
	config := configuration.Config{}

	err := ValidateConfiguration(config)
	assert.Error(t, err)
	assert.Equal(t, "config file must provide at least one system configuration inside the [SystemConfigs] field", err.Error())
}

func TestShouldFailEmptySystemConfig(t *testing.T) {
	config := configuration.Config{}
	config.SystemConfigs = []configuration.SystemConfig{{}}

	err := ValidateConfiguration(config)
	assert.Error(t, err)
	assert.Equal(t, "invalid [SystemConfigs]: missing [Name] field", err.Error())
}

func TestShouldFailDeeplyNestedParsingError(t *testing.T) {
	const (
		configDirectory string = "../../imageconfigs/"
	)
	configFiles, err := ioutil.ReadDir(configDirectory)
	assert.NoError(t, err)

	// Pick the first config file and mess something up which is deeply
	// nested inside the json
	for _, file := range configFiles {
		if !file.IsDir() && strings.Contains(file.Name(), "core-efi.json") {
			configPath := filepath.Join(configDirectory, file.Name())

			fmt.Println("Corrupting ", configPath)

			config, err := configuration.Load(configPath)
			assert.NoError(t, err)

			config.Disks[0].PartitionTableType = configuration.PartitionTableType("not_a_real_partition_type")
			err = ValidateConfiguration(config)
			assert.Error(t, err)
			assert.Equal(t, "invalid [Disks]: invalid [PartitionTableType]: invalid value for PartitionTableType (not_a_real_partition_type)", err.Error())

			return
		}
	}
	assert.Fail(t, "Could not find 'core-efi.json' to test")
}
