// Copyright Microsoft Corporation.
// Licensed under the MIT License.

package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//TestMain found in configuration_test.go.

var (
	validSystemConfig       SystemConfig = expectedConfiguration.SystemConfigs[0]
	invalidSystemConfigJSON              = `{"IsDefault": 1234}`
)

func TestShouldFailParsingDefaultSystemConfig_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig
	err := marshalJSONString("{}", &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: missing [Name] field", err.Error())
}

func TestShouldSucceedParseValidSystemConfig_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	assert.NoError(t, validSystemConfig.IsValid())
	err := remarshalJSON(validSystemConfig, &checkedSystemConfig)
	assert.NoError(t, err)
	assert.Equal(t, validSystemConfig, checkedSystemConfig)
}

func TestShouldFailParsingMissingName_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	missingNameConfig := validSystemConfig
	missingNameConfig.Name = ""

	err := missingNameConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "missing [Name] field", err.Error())

	err = remarshalJSON(missingNameConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: missing [Name] field", err.Error())
}

func TestShouldFailParsingMissingPackages_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	missingPackageListConfig := validSystemConfig
	missingPackageListConfig.PackageLists = []string{}

	err := missingPackageListConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "system configuration must provide at least one package list inside the [PackageLists] field", err.Error())

	err = remarshalJSON(missingPackageListConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: system configuration must provide at least one package list inside the [PackageLists] field", err.Error())
}

func TestShouldFailParsingMissingDefaultKernel_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	missingDefaultConfig := validSystemConfig
	missingDefaultConfig.KernelOptions = map[string]string{}

	err := missingDefaultConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "system configuration must always provide default kernel inside the [KernelOptions] field; remember that kernels are FORBIDDEN from appearing in any of the [PackageLists]", err.Error())

	err = remarshalJSON(missingDefaultConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: system configuration must always provide default kernel inside the [KernelOptions] field; remember that kernels are FORBIDDEN from appearing in any of the [PackageLists]", err.Error())
}

func TestShouldFailParsingMissingExtraBlankKernel_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	blankKernelConfig := validSystemConfig
	// Create a new map so we don't affect other tests
	blankKernelConfig.KernelOptions = map[string]string{"default": "kernel", "extra": ""}

	err := blankKernelConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "empty kernel entry found in the [KernelOptions] field (extra); remember that kernels are FORBIDDEN from appearing in any of the [PackageLists]", err.Error())

	err = remarshalJSON(blankKernelConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: empty kernel entry found in the [KernelOptions] field (extra); remember that kernels are FORBIDDEN from appearing in any of the [PackageLists]", err.Error())
}

func TestShouldSucceedParsingMissingDefaultKernelForRootfs_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	rootfsNoKernelConfig := validSystemConfig
	rootfsNoKernelConfig.KernelOptions = map[string]string{}
	rootfsNoKernelConfig.PartitionSettings = []PartitionSetting{}
	// We can't support a verity root without a root mount.
	rootfsNoKernelConfig.ReadOnlyVerityRoot.Enable = false

	assert.NoError(t, rootfsNoKernelConfig.IsValid())
	err := remarshalJSON(rootfsNoKernelConfig, &checkedSystemConfig)
	assert.NoError(t, err)
	assert.Equal(t, rootfsNoKernelConfig, checkedSystemConfig)
}

func TestShouldFailParsingBadKernelCommandLine_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	badKernelCommandConfig := validSystemConfig
	badKernelCommandConfig.KernelCommandLine = KernelCommandLine{ExtraCommandLine: invalidExtraCommandLine}

	err := badKernelCommandConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [KernelCommandLine]: ExtraCommandLine contains character ` which is reserved for use by sed", err.Error())

	err = remarshalJSON(badKernelCommandConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: failed to parse [KernelCommandLine]: ExtraCommandLine contains character ` which is reserved for use by sed", err.Error())
}

func TestShouldFailToParsingMultipleSameMounts_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	badPartitionSettingsConfig := validSystemConfig
	badPartitionSettingsConfig.PartitionSettings = []PartitionSetting{
		{MountPoint: "/"},
		{MountPoint: "/"},
	}

	err := badPartitionSettingsConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [PartitionSettings]: duplicate mount point found at '/'", err.Error())

	err = remarshalJSON(badPartitionSettingsConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: invalid [PartitionSettings]: duplicate mount point found at '/'", err.Error())
}

func TestShouldFailParsingBothVerityAndEncryption_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	badBothEncryptionVerityConfig := validSystemConfig
	badBothEncryptionVerityConfig.ReadOnlyVerityRoot = validSystemConfig.ReadOnlyVerityRoot
	badBothEncryptionVerityConfig.ReadOnlyVerityRoot.Enable = true

	err := badBothEncryptionVerityConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [ReadOnlyVerityRoot]: verity root currently does not support root encryption", err.Error())

	err = remarshalJSON(badBothEncryptionVerityConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: invalid [ReadOnlyVerityRoot]: verity root currently does not support root encryption", err.Error())
}

func TestShouldFailParsingInvalidVerityRoot_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	badPartitionSettingsConfig := validSystemConfig
	badPartitionSettingsConfig.ReadOnlyVerityRoot = ReadOnlyVerityRoot{
		Enable:        true,
		Name:          "test",
		TmpfsOverlays: []string{"/nested", "/nested/folder"},
	}
	// Encryption and Verity currently can't coexist
	badPartitionSettingsConfig.Encryption.Enable = false

	err := badPartitionSettingsConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [ReadOnlyVerityRoot]: failed to validate [TmpfsOverlays], overlays may not overlap each other (/nested)(/nested/folder)", err.Error())

	err = remarshalJSON(badPartitionSettingsConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: failed to parse [ReadOnlyVerityRoot]: failed to validate [TmpfsOverlays], overlays may not overlap each other (/nested)(/nested/folder)", err.Error())
}

func TestShouldFailParsingVerityRootWithNoRoot_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	badPartitionSettingsConfig := validSystemConfig
	// Take only the boot partition (index 0), drop the root (index 1)
	badPartitionSettingsConfig.PartitionSettings = validSystemConfig.PartitionSettings[0:1]
	badPartitionSettingsConfig.ReadOnlyVerityRoot = ReadOnlyVerityRoot{
		Enable: true,
		Name:   "test",
	}

	err := badPartitionSettingsConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [ReadOnlyVerityRoot]: must have a partition mounted at '/'", err.Error())

	err = remarshalJSON(badPartitionSettingsConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: invalid [ReadOnlyVerityRoot]: must have a partition mounted at '/'", err.Error())
}

func TestShouldFailParsingVerityRootWithNoBoot_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	badPartitionSettingsConfig := validSystemConfig
	// Take only the boot partition (index 0), drop the root (index 1)
	badPartitionSettingsConfig.PartitionSettings = validSystemConfig.PartitionSettings[1:2]
	badPartitionSettingsConfig.ReadOnlyVerityRoot = ReadOnlyVerityRoot{
		Enable: true,
		Name:   "test",
	}

	err := badPartitionSettingsConfig.IsValid()
	assert.Error(t, err)
	assert.Equal(t, "invalid [ReadOnlyVerityRoot]: must have a separate partition mounted at '/boot'", err.Error())

	err = remarshalJSON(badPartitionSettingsConfig, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: invalid [ReadOnlyVerityRoot]: must have a separate partition mounted at '/boot'", err.Error())
}

func TestShouldFailToParseInvalidJSON_SystemConfig(t *testing.T) {
	var checkedSystemConfig SystemConfig

	err := marshalJSONString(invalidSystemConfigJSON, &checkedSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, "failed to parse [SystemConfig]: json: cannot unmarshal number into Go struct field IntermediateTypeSystemConfig.IsDefault of type bool", err.Error())

}
