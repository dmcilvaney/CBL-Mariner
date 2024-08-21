#!/bin/bash

set -ex

basedir=/temp/DockerStage/
chrootdir=$basedir/docker-chroot-1
repo_dir=/home/damcilva/repos/temp/CBL-Mariner_TEMP3

#sudo rm -rf $basedir
#mkdir -p $chrootdir

#touch $basedir/chroot-pool.lock

# Mounts to create for each chroot (we only have 1 for now...)
# Use array to make it easier to add more mounts in the future
mounts=()
mounts+=(-v $repo_dir:/repo)
mounts+=(-v /dev:$chrootdir/dev:ro  )
mounts+=(-v /proc:$chrootdir/proc:ro  )
mounts+=(-v devpts:$chrootdir/dev/pts:ro )
mounts+=(-v sysfs:$chrootdir/sys:ro )
mounts+=(-v $repo_dir/out/RPMS/noarch:$chrootdir/localrpms/noarch:rw  )
mounts+=(-v $repo_dir/out/RPMS/x86_64:$chrootdir/localrpms/x86_64:rw  )
mounts+=(-v $repo_dir/out/RPMS/aarch64:$chrootdir/localrpms/aarch64:rw  )
mounts+=(-v $repo_dir/build/rpm_cache/cache:$chrootdir/upstream-cached-rpms:rw  )
mounts+=(-v $repo_dir/build/toolchain_rpms/x86_64:$chrootdir/toolchainrpms/x86_64:rw  )
mounts+=(-v $repo_dir/build/toolchain_rpms/aarch64:$chrootdir/toolchainrpms/aarch64:rw  )
mounts+=(-v $repo_dir/build/toolchain_rpms/noarch:$chrootdir/toolchainrpms/noarch:rw )

docker run  --privileged -it --rm -v $repo_dir:/repo "${mounts[@]}" mcr.microsoft.com/azurelinux/local/buildcontainer:latest bash -c "\
    git config --global --add safe.directory /repo && \
    touch $basedir/chroot-pool.lock && \
    bash --rcfile <(echo 'cd /repo/toolkit') \
"

#sudo make build-packages SPECS_DIR=../SPECS_TEST DAILY_BUILD_ID=lkg REBUILD_TOOLS=y CHROOT_DIR=/temp/DockerStage/ build-packages LOG_LEVEL=debug
