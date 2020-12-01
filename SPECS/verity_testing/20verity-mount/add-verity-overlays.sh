#!/bin/sh

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

overlays=$(getarg rd.verityroot.overlays=)
#"/var/tmp /var/log /var/lib/systemd /var/systemd"
info "Adding tempfs overlays to the root fs (${overlays})"

# rd.verityroot.overlays=...
# rd.verityroot.overlay_device=...
# rd.verityroot.overlay_debug_mount=...

if [ -z "${overlays}" ]; then
    info "No overlays listed, skip overlayfs creation"
    exit 0
fi

overlay_tmpfs="/verity_rw_overlays"
overlay_mount="/overlay_tmpfs_mnt"
mkdir -p ${overlay_tmpfs}

if ismounted ${overlay_tmpfs}; then
    die "${overlay_tmpfs} is already mounted!"
else
    mount -t tmpfs tmpfs -o rw,nodev,nosuid,nouser,size=20% ${overlay_tmpfs}
fi

create_overlay() {
    folder=$1
    info "Creating overlay for ${folder}"
    newroot_folder=$NEWROOT/${folder}

    if [ ! -d ${newroot_folder} ]; then
        warn "${folder} does not exist, cannot create tmpfs mount point there"
        exit 1 
    fi

    overlay_name=$(str_replace ${newroot_folder} '/' '_')
    overlay_dir=$(mktemp -d --tmpdir=${overlay_tmpfs} "${overlay_name}.XXXXXXXXXX")
    working=${overlay_dir}/working
    upper=${overlay_dir}/upper

    mkdir ${working}
    mkdir ${upper}

    mount -t overlay overlay -o rw,lowerdir=${newroot_folder},upperdir=${upper},workdir=${working} ${newroot_folder}
}

for folder in ${overlays}; do
    create_overlay "${folder}"
done

# This must be last in case one of the overlays covers the mount point. The
# overlay view wil only show an empty folder instead of the bind mounted tmpfs.
if getargbool 0 "rd.verity.overlay_debug_mount"; then
    mount --bind ${overlay_tmpfs} $NEWROOT/${overlay_mount} || die
fi


