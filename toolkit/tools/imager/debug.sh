#!/bin/bash
set -e
make -C ../../ go-tools REBUILD_TOOLS=y

/home/damcilva/repos/CBL-Mariner/toolkit/out/tools/imager \
        --build-dir /home/damcilva/repos/CBL-Mariner/toolkit/../build/imagegen/core-efi/workspace \
        --input /home/damcilva/repos/CBL-Mariner/toolkit/imageconfigs/core-efi.json \
        --base-dir=/home/damcilva/repos/CBL-Mariner/toolkit/imageconfigs/ \
        --log-level trace \
        --log-file /home/damcilva/repos/CBL-Mariner/toolkit/../build/logs/imggen/imager.log \
        --local-repo /home/damcilva/repos/CBL-Mariner/toolkit/../build/imagegen/core-efi/package_repo \
        --tdnf-worker /home/damcilva/repos/CBL-Mariner/toolkit/../build/worker/worker_chroot.tar.gz \
        --repo-file=/home/damcilva/repos/CBL-Mariner/toolkit/resources/manifests/image/local.repo \
        --assets /home/damcilva/repos/CBL-Mariner/toolkit/resources/assets/ \
        --output-dir /home/damcilva/repos/CBL-Mariner/toolkit/../build/imagegen/core-efi/imager_output > ./out.txt 2>&1 &
pid=$!

/home/damcilva/go/bin/dlv --listen 127.0.0.1:12345  --api-version=2  --log --only-same-user=false --allow-non-terminal-interactive=true --accept-multiclient attach $pid 