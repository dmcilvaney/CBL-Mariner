// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//TestMain found in configuration_test.go.

var (
	validTargetDisk       TargetDisk = TargetDisk{}
	invalidTargetDiskJSON            = `{"Type": "path", "Value": "/dev/sda", "RaidConfig": "invalid"}`
)

func TestShouldSucceedParsingDefaultTargetDisk_TargetDisk(t *testing.T) {
	var checkedTargetDisk TargetDisk
	err := marshalJSONString("{}", &checkedTargetDisk)
	assert.NoError(t, err)
	assert.Equal(t, TargetDisk{}, checkedTargetDisk)
}

func TestShouldSucceedParsingValidTargetDisk_TargetDisk(t *testing.T) {
	var checkedTargetDisk TargetDisk
	err := remarshalJSON(validTargetDisk, &checkedTargetDisk)
	assert.NoError(t, err)
	assert.Equal(t, validTargetDisk, checkedTargetDisk)
}

func TestShouldFailParsingInvalidJSON_TargetDisk(t *testing.T) {
	var checkedTargetDisk TargetDisk

	err := marshalJSONString(invalidTargetDiskJSON, &checkedTargetDisk)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [TargetDisk]: failed to parse [RaidConfig]: json: cannot unmarshal string into Go value of type configuration.IntermediateTypeRaidConfig", err.Error())
}

func TestShouldPassTypePath_TargetDisk(t *testing.T) {
	var checkedTargetDisk TargetDisk
	pathTargetDisk := validTargetDisk

	pathTargetDisk.Type = TargetDiskTypePath
	pathTargetDisk.Value = "/dev/sda"

	err := pathTargetDisk.IsValid()
	assert.NoError(t, err)

	err = remarshalJSON(pathTargetDisk, &checkedTargetDisk)
	assert.NoError(t, err)
	assert.Equal(t, pathTargetDisk, checkedTargetDisk)
}

func TestShouldFailTypePathNoValue_TargetDisk(t *testing.T) {
	var checkedTargetDisk TargetDisk
	PathTargetDisk := validTargetDisk

	PathTargetDisk.Type = TargetDiskTypePath
	PathTargetDisk.Value = ""

	err := PathTargetDisk.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [TargetDisk]: Value must be specified for TargetDiskType of 'path'", err.Error())

	err = remarshalJSON(PathTargetDisk, &checkedTargetDisk)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [TargetDisk]: invalid [TargetDisk]: Value must be specified for TargetDiskType of 'path'", err.Error())
}

func TestShouldFailInvalidType_TargetDisk(t *testing.T) {
	var checkedTargetDisk TargetDisk
	invalidTargetDisk := validTargetDisk

	invalidTargetDisk.Type = "invalid"

	err := invalidTargetDisk.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [TargetDisk]: invalid value for TargetDiskType (invalid)", err.Error())

	err = remarshalJSON(invalidTargetDisk, &checkedTargetDisk)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [TargetDisk]: failed to parse [TargetDiskType]: invalid value for TargetDiskType (invalid)", err.Error())
}

func TestShouldFailNonEmptyStructWithNoneType_TargetDisk(t *testing.T) {
	var checkedTargetDisk TargetDisk
	invalidTargetDisk := validTargetDisk

	invalidTargetDisk.Type = ""
	invalidTargetDisk.Value = "/dev/sda"

	err := invalidTargetDisk.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [TargetDisk]: Value and RaidConfig must be empty for TargetDiskType of ''", err.Error())

	err = remarshalJSON(invalidTargetDisk, &checkedTargetDisk)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [TargetDisk]: invalid [TargetDisk]: Value and RaidConfig must be empty for TargetDiskType of ''", err.Error())

	invalidTargetDisk.Type = ""
	invalidTargetDisk.Value = ""
	invalidTargetDisk.RaidConfig = RaidConfig{ComponentPartIDs: []string{"1", "2"}}

	err = invalidTargetDisk.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [TargetDisk]: Value and RaidConfig must be empty for TargetDiskType of ''", err.Error())

	err = remarshalJSON(invalidTargetDisk, &checkedTargetDisk)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [TargetDisk]: invalid [TargetDisk]: Value and RaidConfig must be empty for TargetDiskType of ''", err.Error())
}
