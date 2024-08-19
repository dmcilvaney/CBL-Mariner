// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package resources

import (
	"embed"
)

const (
	AssetsGrubCfgFile = "assets/grub2/grub.cfg"
	AssetsGrubDefFile = "assets/grub2/grub"

	AssetsBaseDockerFile           = "assets/docker/base.Dockerfile"
	AssetsSrpmDockerFile           = "assets/docker/srpm.Dockerfile"
	AssetsRpmDockerFile            = "assets/docker/rpm.Dockerfile"
	AssetsCacheDockerFile          = "assets/docker/cache.Dockerfile"
	AssetsCreateRepoAndRunScript   = "assets/docker/create_repos_and_run.sh"
	AssetsRepoFileTemplate         = "assets/docker/local.template"
	AssetsUpstreamRepoFileTemplate = "assets/docker/upstream.template"
)

var DockerAssets = []string{
	AssetsBaseDockerFile,
	AssetsSrpmDockerFile,
	AssetsRpmDockerFile,
	AssetsCacheDockerFile,
	AssetsCreateRepoAndRunScript,
	AssetsRepoFileTemplate,
	AssetsUpstreamRepoFileTemplate,
}

//go:embed assets
var ResourcesFS embed.FS
