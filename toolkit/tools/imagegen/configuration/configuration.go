// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Parser for the image builder's configuration schemas.

package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/file"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/jsonutils"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/logger"
)

// Artifact [non-ISO image building only] defines the name, type
// and optional compression of the output Mariner image.
type Artifact struct {
	Compression string `json:"Compression"`
	Name        string `json:"Name"`
	Type        string `json:"Type"`
}

// RawBinary allow the users to specify a binary they would
// like to copy byte-for-byte onto the disk.
type RawBinary struct {
	BinPath   string `json:"BinPath"`
	BlockSize uint64 `json:"BlockSize"`
	Seek      uint64 `json:"Seek"`
}

// InstallScript defines a script to be run before or after other installation
// steps and provides a way to pass parameters to it.
type InstallScript struct {
	Args string `json:"Args"`
	Path string `json:"Path"`
}

// Group defines a single group to be created on the new system.
type Group struct {
	Name string `json:"Name"`
	GID  string `json:"GID"`
}

// RootEncryption enables encryption on the root partition
type RootEncryption struct {
	Enable   bool   `json:"Enable"`
	Password string `json:"Password"`
}

// Config holds the parsed values of the configuration schemas as well as
// a few computed values simplifying access to certain pieces of the configuration.
type Config struct {
	// Values representing the contents of the config JSON file.
	Disks         []Disk         `json:"Disks"`
	SystemConfigs []SystemConfig `json:"SystemConfigs"`

	// Computed values not present in the config JSON.
	DefaultSystemConfig *SystemConfig // A system configuration with the "IsDefault" field set or the first system configuration if there is no explicit default.
}

// GetDiskPartByID returns the disk partition object with the desired ID, nil if no partition found
func (c *Config) GetDiskPartByID(ID string) (diskPart *Partition) {
	for i, d := range c.Disks {
		for j, p := range d.Partitions {
			if p.ID == ID {
				return &c.Disks[i].Partitions[j]
			}
		}
	}
	return nil
}

// GetDiskByPartition returns the disk containing the provided partition
func (c *Config) GetDiskContainingPartition(partition *Partition) (disk *Disk) {
	ID := partition.ID
	for i, d := range c.Disks {
		for _, p := range d.Partitions {
			if p.ID == ID {
				return &c.Disks[i]
			}
		}
	}
	return nil
}

func (c *Config) GetBootPartition() (partitionIndex int, partition *Partition) {
	for i, d := range c.Disks {
		for j, p := range d.Partitions {
			if p.HasFlag(PartitionFlagBoot) {
				return j, &c.Disks[i].Partitions[j]
			}
		}
	}
	return
}

// addDiskToOrdering adds the provided disk to the ordering, taking into account any dependencies. This is a recursive function.
// The ordering is a list of disks that can be created in order, with the first disk in the list being the first to be created.
func (c *Config) addDiskToOrdering(disk *Disk, orderedDisks []Disk, haveAddedDisk map[*Disk]bool) (newOrderedDisks []Disk, err error) {
	if haveAddedDisk[disk] {
		return orderedDisks, nil
	}

	switch disk.TargetDisk.Type {
	case TargetDiskTypeRaid:
		// For RAID disks, we need to add the disks they depend on first.
		for _, raidDiskPartID := range disk.TargetDisk.RaidConfig.ComponentPartIDs {
			raidDiskPart := c.GetDiskPartByID(raidDiskPartID)
			if raidDiskPart == nil {
				err = fmt.Errorf("failed to find RAID component part with ID '%s'", raidDiskPartID)
				return
			}

			raidDiskPartParentDisk := c.GetDiskContainingPartition(raidDiskPart)
			if raidDiskPartParentDisk == nil {
				err = fmt.Errorf("failed to find RAID parent disk for partition with ID '%s'", raidDiskPartID)
				return
			}

			orderedDisks, err = c.addDiskToOrdering(raidDiskPartParentDisk, orderedDisks, haveAddedDisk)
			if err != nil {
				err = fmt.Errorf("failed to add RAID component disk '%s' to ordering: %s", raidDiskPartParentDisk.ID, err.Error())
				return
			}
		}
		fallthrough // Fall through to the common case since we can now add the RAID disk itself.
	case TargetDiskTypePath, TargetDiskTypeNone:
		orderedDisks = append(orderedDisks, *disk)
		haveAddedDisk[disk] = true
	default:
		err = fmt.Errorf("unsupported disk type '%s', can't determine order", disk.TargetDisk.Type)
	}
	return orderedDisks, err
}

// GetDiskCreationOrder returns a list of disks that need to be created, in the order they need to be created to avoid dependency issues.
// If config is empty, returns an empty list.
func (c *Config) GetDiskCreationOrder() (orderedDisks []Disk, err error) {
	// First sort the disks by their mount point so nested mounts will work correctly.

	// A map of disks that have already been added to the ordering so we don't add them twice during recursion.
	haveAddedDisk := make(map[*Disk]bool)
	for idx, _ := range c.Disks {
		// Add each disk to the ordering. This is a recursive function that will add all dependencies for a disk first if needed.
		orderedDisks, err = c.addDiskToOrdering(&c.Disks[idx], orderedDisks, haveAddedDisk)
		if err != nil {
			return nil, err
		}
	}
	return
}

// GetKernelCmdLineValue returns the output of a specific option setting in /proc/cmdline
func GetKernelCmdLineValue(option string) (cmdlineValue string, err error) {
	const cmdlineFile = "/proc/cmdline"

	content, err := os.ReadFile(cmdlineFile)
	if err != nil {
		logger.Log.Errorf("failed to read from %s", cmdlineFile)
		return
	}

	cmdline := string(content)
	if strings.Count(cmdline, option) > 1 {
		err = fmt.Errorf("/proc/cmdline contains duplicate (%s) entries, which is invalid", option)
		return
	}

	cmdlineArgs := strings.Split(cmdline, " ")
	for _, cmdlineArg := range cmdlineArgs {
		if strings.Contains(cmdlineArg, option) {
			cmdlineValue = cmdlineArg[(len(option) + 1):len(cmdlineArg)]
			return
		}
	}

	return
}

// checkDeviceMapperFlags checks if Encryption and read-only roots have the required 'dmroot' flag.
// They need the root partition to have a specific flag so we can find the partition and handle it
// before we mount it.
func checkDeviceMapperFlags(config *Config) (err error) {
	for _, sysConfig := range config.SystemConfigs {
		var dmRoot *Partition
		if sysConfig.ReadOnlyVerityRoot.Enable || sysConfig.Encryption.Enable {
			if len(config.Disks) == 0 {
				logger.Log.Warnf("[ReadOnlyVerityRoot] or [Encryption] is enabled, but no partitions are listed as part of System Config '%s'. This is only valid for ISO installers", sysConfig.Name)
				continue
			}

			rootPartSetting := sysConfig.GetRootPartitionSetting()
			if rootPartSetting == nil {
				return fmt.Errorf("can't find a root ('/') [PartitionSetting] to work with either [ReadOnlyVerityRoot] or [Encryption]")
			}
			rootDiskPart := config.GetDiskPartByID(rootPartSetting.ID)
			if rootDiskPart == nil {
				return fmt.Errorf("can't find a [Disk] [Partition] to match with [PartitionSetting] '%s'", rootPartSetting.ID)
			}
			if !rootDiskPart.HasFlag(PartitionFlagDeviceMapperRoot) {
				return fmt.Errorf("[Partition] '%s' must include 'dmroot' device mapper root flag in [Flags] for [SystemConfig] '%s's root partition since it uses [ReadOnlyVerityRoot] or [Encryption]", rootDiskPart.ID, sysConfig.Name)
			}
		}
		// There is currently a limitation in diskutils.CreatePartitions() which requires us to know our device-mapper
		// partitions prior to parsing the systemconfigs. We won't know if a given systemconfig will require
		// the device mapper root functionality for the "/" mount point ahead of time, nor which partition will
		// be mounted there. Make sure we only have one such root to choose from at any given time so we won't
		// confuse them.
		for _, partSetting := range sysConfig.PartitionSettings {
			part := config.GetDiskPartByID(partSetting.ID)
			if part != nil && part.HasFlag(PartitionFlagDeviceMapperRoot) {
				if dmRoot != nil {
					return fmt.Errorf("[SystemConfig] '%s' includes two (or more) device mapper root [PartitionSettings] '%s' and '%s', include only one", sysConfig.Name, dmRoot.ID, part.ID)
				}
				dmRoot = part
			}
		}
	}
	return
}

// checkForMissingDiskPartitions makes sure we don't try to mount a partition which doesn't actually exist as part of a disk.
func checkForMissingDiskPartitions(config *Config) (err error) {
	for _, sysConfig := range config.SystemConfigs {
		for _, partSetting := range sysConfig.PartitionSettings {
			if config.GetDiskPartByID(partSetting.ID) == nil {
				return fmt.Errorf("[SystemConfig] '%s' mounts a [Partition] '%s' which has no corresponding partition on a [Disk]", sysConfig.Name, partSetting.ID)
			}
		}
	}
	return
}

// checkDuplicatePartitionIDs makes sure that we don't have two disk partitions which share an ID. It would not be clear
// which should be mounted by a SystemConfig.
func checkDuplicatePartitionIDs(config *Config) (err error) {
	idUsed := make(map[string]int)
	for i, disk := range config.Disks {
		for _, part := range disk.Partitions {
			id := part.ID
			otherDisk, alreadyUsed := idUsed[id]
			if alreadyUsed {
				return fmt.Errorf("a [Partition] on a [Disk] '%d' shares an ID '%s' with another partition (on disk '%d')", otherDisk, id, i)
			} else {
				idUsed[id] = i
			}
		}
	}
	return
}

// checkInvalidMountIdentifiers checks that we don't have an invalid combination of GPT/MBR, PARTLABEL, and Name for each partition.
// PARTUUID and PARTLABEL are GPT concepts. MBR partly supports PARTUUID, but is completely incompatible with PARTLABEL.
// If we want to use PARTLABEL, we need to define a [Name] for the partition as well. RAID disks cannot support PART* identifiers and
// need to use UUID.
func checkInvalidMountIdentifiers(config *Config) (err error) {
	for _, sysConfig := range config.SystemConfigs {
		for _, partSetting := range sysConfig.PartitionSettings {
			diskPart := config.GetDiskPartByID(partSetting.ID)
			disk := config.GetDiskContainingPartition(diskPart)

			if partSetting.MountIdentifier == MountIdentifierPartLabel {
				if disk.PartitionTableType != PartitionTableTypeGpt {
					return fmt.Errorf("[SystemConfig] '%s' mounts a [Partition] '%s' via PARTLABEL, but that partition is on an MBR disk which does not support PARTLABEL", sysConfig.Name, partSetting.ID)
				}

				if diskPart.Name == "" {
					return fmt.Errorf("[SystemConfig] '%s' mounts a [Partition] '%s' via PARTLABEL, but it has no [Name]", sysConfig.Name, partSetting.ID)
				}
			}

			if disk.TargetDisk.Type == TargetDiskTypeRaid && partSetting.MountIdentifier != MountIdentifierUuid {
				return fmt.Errorf("[SystemConfig] '%s' mounts a [Partition] '%s' on a RAID disk, but it is not mounted via UUID. RAID only supports [MountIdentifier]='%s'", sysConfig.Name, partSetting.ID, MountIdentifierUuid)
			}
		}
	}
	return
}

func checkInvalidMultiDiskConfig(config *Config) (err error) {
	if len(config.Disks) > 1 {
		for _, sysConfig := range config.SystemConfigs {
			if sysConfig.PrimaryDisk == "" {
				return fmt.Errorf("[SystemConfig] '%s' PrimaryDisk value required configuring for multiple disks", sysConfig.Name)
			} else {
				for _, disk := range config.Disks {
					if disk.ID == sysConfig.PrimaryDisk {
						return err
					}
				}
				return fmt.Errorf("[SystemConfig] '%s' no matching disk ID found for PrimaryDisk '%s'", sysConfig.Name, sysConfig.PrimaryDisk)
			}
		}
	}
	return err
}

// Can't have disks with both paths set and not set. Special types like RAID work fine with either.
func checkMismatchedDiskTypes(config *Config) (err error) {
	foundTargetTypeNone := false
	foundTargetTypePath := false
	if len(config.Disks) > 1 {
		for i := 1; i < len(config.Disks); i++ {
			if config.Disks[i].TargetDisk.Type == TargetDiskTypeNone {
				foundTargetTypeNone = true
			} else if config.Disks[i].TargetDisk.Type == TargetDiskTypePath {
				foundTargetTypePath = true
			}
		}
	}
	if foundTargetTypeNone && foundTargetTypePath {
		return fmt.Errorf("cannot have a [Disk] with TargetDisk of Type '%s' mixed with disks that have a TargetDisk Type '%s'", TargetDiskTypeNone, TargetDiskTypePath)
	} else {
		return nil
	}
}

func checkDuplicateArtifactNames(config *Config) (err error) {
	artifactNames := make(map[string]bool)
	for i, disk := range config.Disks {
		for _, artifact := range disk.Artifacts {
			filename := artifact.Name + artifact.Type
			if _, exists := artifactNames[filename]; exists {
				return fmt.Errorf("[Disk] '%d' has an [Artifact] with a duplicate name '%s' and type '%s'", i, artifact.Name, artifact.Type)
			}
			artifactNames[filename] = true
		}
	}
	return err
}

func checkRaidDisk(config *Config, disk *Disk) (err error) {
	// Make sure that all the component partitions exist
	for _, partID := range disk.TargetDisk.RaidConfig.ComponentPartIDs {
		if config.GetDiskPartByID(partID) == nil {
			return fmt.Errorf("RAID component partition '%s' does not exist", partID)
		}
	}

	if len(disk.Partitions) > 1 {
		return fmt.Errorf("RAID disk '%s' cannot have more than one [Partition] per RAID disk (found %d)", disk.ID, len(disk.Partitions))
	}

	if disk.Partitions[0].Start != 0 || disk.Partitions[0].End != 0 {
		return fmt.Errorf("RAID disk '%s' cannot have a [Partition] with a Start or End value", disk.ID)
	}

	return
}

// validateAllRaidDisks makes sure that all RAID disks have a unique ID and that the RAID configuration is valid.
func validateAllRaidDisks(config *Config) (err error) {
	raidIDs := make(map[string]bool)
	for _, disk := range config.Disks {
		if disk.TargetDisk.Type == TargetDiskTypeRaid {
			// Check the config
			err = checkRaidDisk(config, &disk)
			if err != nil {
				return fmt.Errorf("invalid [Disks] RAID configuration: %w", err)
			}

			// Check for duplicate IDs
			raidID := disk.TargetDisk.RaidConfig.RaidID
			if raidIDs[raidID] {
				return fmt.Errorf("found multiple [Disks] with the same [Raid] ID '%s'", raidID)
			}
			raidIDs[raidID] = true
		}
	}
	return
}

// IsValid returns an error if the Config is not valid
func (c *Config) IsValid() (err error) {
	for _, disk := range c.Disks {
		if err = disk.IsValid(); err != nil {
			return fmt.Errorf("invalid [Disks]: %w", err)
		}
	}

	// Check that we will be able to reliably find our disk partitions for each SystemConfig
	err = checkForMissingDiskPartitions(c)
	if err != nil {
		return fmt.Errorf("invalid [Config]: %w", err)
	}

	err = checkDuplicatePartitionIDs(c)
	if err != nil {
		return fmt.Errorf("invalid [Config]: %w", err)
	}

	// Check the flags for the disks
	err = checkDeviceMapperFlags(c)
	if err != nil {
		return fmt.Errorf("a config in [SystemConfigs] enables a device mapper based root (Encryption or Read-Only), but partitions are miss-configured: %w", err)
	}

	err = checkInvalidMountIdentifiers(c)
	if err != nil {
		return fmt.Errorf("invalid [Config]: %w", err)
	}

	err = checkMismatchedDiskTypes(c)
	if err != nil {
		return fmt.Errorf("invalid [Config]: %w", err)
	}

	err = checkInvalidMultiDiskConfig(c)
	if err != nil {
		return fmt.Errorf("invalid [Config]: %w", err)
	}

	err = checkDuplicateArtifactNames(c)
	if err != nil {
		return fmt.Errorf("invalid [Config]: %w", err)
	}

	err = validateAllRaidDisks(c)
	if err != nil {
		return fmt.Errorf("invalid [Config]: %w", err)
	}

	if len(c.SystemConfigs) == 0 {
		return fmt.Errorf("config file must provide at least one system configuration inside the [SystemConfigs] field")
	}
	for _, sysConfig := range c.SystemConfigs {
		if err = sysConfig.IsValid(); err != nil {
			return fmt.Errorf("invalid [SystemConfigs]: %w", err)
		}
	}
	defaultFound := false
	for _, sysConfig := range c.SystemConfigs {
		if sysConfig.IsDefault {
			if defaultFound {
				return fmt.Errorf("config file must have no more than one default system configuration. Please remove redundant [IsDefault] fields")
			}
			defaultFound = true
		}
	}
	return
}

// UnmarshalJSON Unmarshals a Config entry
func (c *Config) UnmarshalJSON(b []byte) (err error) {
	// Use an intermediate type which will use the default JSON unmarshal implementation
	type IntermediateTypeConfig Config
	err = json.Unmarshal(b, (*IntermediateTypeConfig)(c))
	if err != nil {
		return fmt.Errorf("failed to parse [Config]: %w", err)
	}

	// Now validate the resulting unmarshaled object
	err = c.IsValid()
	if err != nil {
		return fmt.Errorf("failed to parse [Config]: %w", err)
	}
	return
}

// Load loads the config schema from a JSON file found under the 'configFilePath'.
func Load(configFilePath string) (config Config, err error) {
	logger.Log.Debugf("Reading config file from '%s'.", configFilePath)

	err = jsonutils.ReadJSONFile(configFilePath, &config)
	if err != nil {
		return
	}

	config.SetDefaultConfig()

	return
}

// LoadWithAbsolutePaths loads the config schema from a JSON file found under the 'configFilePath'
// and resolves all relative paths into absolute ones using 'baseDirPath' as a starting point for all
// relative paths.
func LoadWithAbsolutePaths(configFilePath, baseDirPath string) (config Config, err error) {
	config, err = Load(configFilePath)
	if err != nil {
		return
	}

	baseDirPath, err = resolveBaseDirPath(baseDirPath, configFilePath)
	if err != nil {
		logger.Log.Errorf("Failed to resolve base directory path (%s) for config under (%s)", baseDirPath, configFilePath)
		return
	}

	config.convertToAbsolutePaths(baseDirPath)

	return
}

// convertToAbsolutePaths converts all of the config's local file paths into absolute ones.
func (c *Config) convertToAbsolutePaths(baseDirPath string) {
	for i := range c.Disks {
		diskConfig := &c.Disks[i]
		convertRawBinariesPath(baseDirPath, diskConfig)
	}

	for i := range c.SystemConfigs {
		systemConfig := &c.SystemConfigs[i]

		convertAdditionalFilesPath(baseDirPath, systemConfig)
		convertPackageListPaths(baseDirPath, systemConfig)
		convertPreInstallScriptsPaths(baseDirPath, systemConfig)
		convertPostInstallScriptsPaths(baseDirPath, systemConfig)
		convertFinalizeImageScriptsPaths(baseDirPath, systemConfig)
		convertSSHPubKeys(baseDirPath, systemConfig)
	}
}

func convertRawBinariesPath(baseDirPath string, diskConfig *Disk) {
	for i, rawBinary := range diskConfig.RawBinaries {
		diskConfig.RawBinaries[i].BinPath = file.GetAbsPathWithBase(baseDirPath, rawBinary.BinPath)
	}
}

func convertAdditionalFilesPath(baseDirPath string, systemConfig *SystemConfig) {
	absAdditionalFiles := make(map[string]string)
	for localFilePath, targetFilePath := range systemConfig.AdditionalFiles {
		localFilePath = file.GetAbsPathWithBase(baseDirPath, localFilePath)
		absAdditionalFiles[localFilePath] = targetFilePath
	}
	systemConfig.AdditionalFiles = absAdditionalFiles
}

func convertPackageListPaths(baseDirPath string, systemConfig *SystemConfig) {
	for i, packageListPath := range systemConfig.PackageLists {
		systemConfig.PackageLists[i] = file.GetAbsPathWithBase(baseDirPath, packageListPath)
	}
}

func convertPreInstallScriptsPaths(baseDirPath string, systemConfig *SystemConfig) {
	for i, preInstallScript := range systemConfig.PreInstallScripts {
		systemConfig.PreInstallScripts[i].Path = file.GetAbsPathWithBase(baseDirPath, preInstallScript.Path)
	}
}

func convertPostInstallScriptsPaths(baseDirPath string, systemConfig *SystemConfig) {
	for i, postInstallScript := range systemConfig.PostInstallScripts {
		systemConfig.PostInstallScripts[i].Path = file.GetAbsPathWithBase(baseDirPath, postInstallScript.Path)
	}
}

func convertFinalizeImageScriptsPaths(baseDirPath string, systemConfig *SystemConfig) {
	for i, finalizeImageScript := range systemConfig.FinalizeImageScripts {
		systemConfig.FinalizeImageScripts[i].Path = file.GetAbsPathWithBase(baseDirPath, finalizeImageScript.Path)
	}
}

func convertSSHPubKeys(baseDirPath string, systemConfig *SystemConfig) {
	for _, user := range systemConfig.Users {
		for i, sshKeyPath := range user.SSHPubKeyPaths {
			user.SSHPubKeyPaths[i] = file.GetAbsPathWithBase(baseDirPath, sshKeyPath)
		}
	}
}

// resolveBaseDirPath returns an absolute path to the base directory or
// the absolute path to the config file directory if `baseDirPath` is empty.
func resolveBaseDirPath(baseDirPath, configFilePath string) (absoluteBaseDirPath string, err error) {
	if baseDirPath == "" {
		baseDirPath = filepath.Dir(configFilePath)
	}

	return filepath.Abs(baseDirPath)
}

func (c *Config) SetDefaultConfig() {
	c.DefaultSystemConfig = &c.SystemConfigs[0]
	for i, systemConfig := range c.SystemConfigs {
		if systemConfig.IsDefault {
			c.DefaultSystemConfig = &c.SystemConfigs[i]
			return
		}
	}
}
