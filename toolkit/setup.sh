#!/bin/bash

set -ex

script_dir=$(dirname $0)
workdir=$(pwd)/mariner_workdir
sudo rm -rf $workdir

chroot_dir=/temp/DockerStage/
basedir=$chroot_dir/docker-chroot-
repo_dir=/home/mariner_user/CBL-Mariner

mounts=()
mounts+=(-v $workdir:/tmp/mariner)
mounts+=(-v $repo_dir:/repo)

num_chroots=10
for i in $(seq 1 $num_chroots); do
    chrootdir=$basedir$i
    mounts+=(-v /dev:$chrootdir/dev:ro  )
    mounts+=(-v /proc:$chrootdir/proc:ro  )
    mounts+=(-v devpts:$chrootdir/dev/pts:ro )
    mounts+=(-v sysfs:$chrootdir/sys:ro )
    mounts+=(-v $workdir/out/RPMS/noarch:$chrootdir/localrpms/noarch:rw  )
    mounts+=(-v $workdir/out/RPMS/x86_64:$chrootdir/localrpms/x86_64:rw  )
    mounts+=(-v $workdir/out/RPMS/aarch64:$chrootdir/localrpms/aarch64:rw  )
    mounts+=(-v $workdir/build/rpm_cache/cache:$chrootdir/upstream-cached-rpms:rw  )
    mounts+=(-v $workdir/build/toolchain_rpms/x86_64:$chrootdir/toolchainrpms/x86_64:rw  )
    mounts+=(-v $workdir/build/toolchain_rpms/aarch64:$chrootdir/toolchainrpms/aarch64:rw  )
    mounts+=(-v $workdir/build/toolchain_rpms/noarch:$chrootdir/toolchainrpms/noarch:rw )
done

# cd to docker build folder and ensure image is up to date
docker build $script_dir/engdocker -t mcr.microsoft.com/azurelinux/local/buildcontainer

docker run  --privileged -it --rm -v $repo_dir:/repo "${mounts[@]}" mcr.microsoft.com/azurelinux/local/buildcontainer:latest bash -c "\
    git config --global --add safe.directory /repo && \
    touch $chroot_dir/chroot-pool.lock && \
    bash --rcfile <(echo 'cd /repo/toolkit') \
"

#sudo make build-packages SPECS_DIR=../SPECS_TEST DAILY_BUILD_ID=lkg REBUILD_TOOLS=y CHROOT_DIR=/temp/DockerStage/ build-packages LOG_LEVEL=debug BUILD_DIR=/tmp/mariner/build OUT_DIR=/tmp/mariner/out -j10
