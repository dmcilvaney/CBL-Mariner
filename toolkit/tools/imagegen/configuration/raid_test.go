// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//TestMain found in configuration_test.go.

var (
	validRaidConfig       RaidConfig = RaidConfig{RaidID: "MyRaidID", ComponentPartIDs: []string{"MyPartID1", "MyPartID2"}}
	validRaidConfigJSON              = `{"RaidID": "MyRaidID", "ComponentPartIDs": ["MyPartID1", "MyPartID2"]}`
	invalidRaidConfigJSON            = `{"RaidID": 0}`
)

func TestShouldSucceedParsingDefaultRaidConfig_RaidConfig(t *testing.T) {
	var checkedRaidConfig RaidConfig
	err := marshalJSONString("{}", &checkedRaidConfig)
	assert.NoError(t, err)
	assert.Equal(t, RaidConfig{}, checkedRaidConfig)
}

func TestShouldSucceedParsingValidRaidConfig_RaidConfig(t *testing.T) {
	var checkedRaidConfig RaidConfig
	err := remarshalJSON(validRaidConfig, &checkedRaidConfig)
	assert.NoError(t, err)
	assert.Equal(t, validRaidConfig, checkedRaidConfig)
}

func TestShouldSucceedParsingValidJSON_RaidConfig(t *testing.T) {
	var checkedRaidConfig RaidConfig

	err := marshalJSONString(validRaidConfigJSON, &checkedRaidConfig)
	assert.NoError(t, err)
	assert.Equal(t, validRaidConfig, checkedRaidConfig)
}

func TestShouldFailParsingInvalidJSON_RaidConfig(t *testing.T) {
	var checkedRaidConfig RaidConfig

	err := marshalJSONString(invalidRaidConfigJSON, &checkedRaidConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [RaidConfig]: json: cannot unmarshal number into Go struct field IntermediateTypeRaidConfig.RaidID of type string", err.Error())
}

// func TestShouldPassTypePath_RaidConfig(t *testing.T) {
// 	var checkedRaidConfig RaidConfig
// 	pathRaidConfig := validRaidConfig

// 	pathRaidConfig.Type = RaidConfigTypePath
// 	pathRaidConfig.Value = "/dev/sda"

// 	err := pathRaidConfig.IsValid()
// 	assert.NoError(t, err)

// 	err = remarshalJSON(pathRaidConfig, &checkedRaidConfig)
// 	assert.NoError(t, err)
// 	assert.Equal(t, pathRaidConfig, checkedRaidConfig)
// }

// func TestShouldFailTypePathNoValue_RaidConfig(t *testing.T) {
// 	var checkedRaidConfig RaidConfig
// 	PathRaidConfig := validRaidConfig

// 	PathRaidConfig.Type = RaidConfigTypePath
// 	PathRaidConfig.Value = ""

// 	err := PathRaidConfig.IsValid()
// 	assert.Error(t, err)
// 	assert.Equal(t, "invalid [RaidConfig]: Value must be specified for RaidConfigType of 'path'", err.Error())

// 	err = remarshalJSON(PathRaidConfig, &checkedRaidConfig)
// 	assert.Error(t, err)
// 	assert.Equal(t, "failed to parse [RaidConfig]: invalid [RaidConfig]: Value must be specified for RaidConfigType of 'path'", err.Error())
// }

// func TestShouldFailInvalidType_RaidConfig(t *testing.T) {
// 	var checkedRaidConfig RaidConfig
// 	invalidRaidConfig := validRaidConfig

// 	invalidRaidConfig.Type = "invalid"

// 	err := invalidRaidConfig.IsValid()
// 	assert.Error(t, err)
// 	assert.Equal(t, "invalid [RaidConfig]: invalid value for RaidConfigType (invalid)", err.Error())

// 	err = remarshalJSON(invalidRaidConfig, &checkedRaidConfig)
// 	assert.Error(t, err)
// 	assert.Equal(t, "failed to parse [RaidConfig]: failed to parse [RaidConfigType]: invalid value for RaidConfigType (invalid)", err.Error())
// }

// func TestShouldFailNonEmptyStructWithNoneType_RaidConfig(t *testing.T) {
// 	var checkedRaidConfig RaidConfig
// 	invalidRaidConfig := validRaidConfig

// 	invalidRaidConfig.Type = ""
// 	invalidRaidConfig.Value = "/dev/sda"

// 	err := invalidRaidConfig.IsValid()
// 	assert.Error(t, err)
// 	assert.Equal(t, "invalid [RaidConfig]: Value and RaidConfig must be empty for RaidConfigType of ''", err.Error())

// 	err = remarshalJSON(invalidRaidConfig, &checkedRaidConfig)
// 	assert.Error(t, err)
// 	assert.Equal(t, "failed to parse [RaidConfig]: invalid [RaidConfig]: Value and RaidConfig must be empty for RaidConfigType of ''", err.Error())

// 	invalidRaidConfig.Type = ""
// 	invalidRaidConfig.Value = ""
// 	invalidRaidConfig.RaidConfig = RaidConfig{ComponentPartIDs: []string{"1", "2"}}

// 	err = invalidRaidConfig.IsValid()
// 	assert.Error(t, err)
// 	assert.Equal(t, "invalid [RaidConfig]: Value and RaidConfig must be empty for RaidConfigType of ''", err.Error())

// 	err = remarshalJSON(invalidRaidConfig, &checkedRaidConfig)
// 	assert.Error(t, err)
// 	assert.Equal(t, "failed to parse [RaidConfig]: invalid [RaidConfig]: Value and RaidConfig must be empty for RaidConfigType of ''", err.Error())
// }

// func TestShouldFailRaidConfigNoComponents(t *testing.T) {
// 	var checkedRaidConfig RaidConfig
// 	invalidRaidConfig := validRaidConfig

// 	invalidRaidConfig.Type = RaidConfigTypeRaid
// 	invalidRaidConfig.RaidConfig = RaidConfig{RaidID: "newRaidID", ComponentPartIDs: []string{}}

// 	err := invalidRaidConfig.IsValid()
// 	assert.Error(t, err)
// 	assert.Equal(t, "invalid [RaidConfig]: Raid 'newRaidID' must have non-empty ComponentPartIDs", err.Error())

// 	err = remarshalJSON(invalidRaidConfig, &checkedRaidConfig)
// 	assert.Error(t, err)
// 	assert.Equal(t, "failed to parse [RaidConfig]: invalid [RaidConfig]: Raid 'newRaidID' must have non-empty ComponentPartIDs", err.Error())
// }
