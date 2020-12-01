#!/bin/sh

# Make sure we have dracut-lib loaded
type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

VERITY_MOUNT="/verity_root/verity_mnt"
OVERLAY_TMPFS="/verity_root/overlays"
OVERLAY_MNT_OPTS="rw,nodev,nosuid,nouser,noexec"

# Get verity root. This should already be set by the dracut cmdline module
[ -n "$root" ] || root=$(getarg root=)
# Bail early if no 'verityroot' root is found
[ "${root%%:*}" = "verityroot" ] || exit 0
verityroot="$root"

# Get the rest of the parameters
[ -z "$verityhashtree" ] && verityhashtree=$(getarg rd.verityroot.hashtree=)
[ -z "$verityroothash" ] && verityroothash=$(getarg rd.verityroot.roothash=)
[ -z "$veritydevice" ] && veritydevice=$(getarg rd.verityroot.devicename=)
[ -n "$veritydevice" ] || veritydevice="verity_root"

[ -z "$overlays" ] && verity_overlays=$(getarg rd.verityroot.overlays=)
[ -z "$overlays_debug_mount" ] && overlays_debug_mount=$(getarg rd.verityroot.overlays_debug_mount=)

info "verityroot='$verityroot'"
info "verityhashtree='$verityhashtree'"
info "verityroothash='$verityroothash'"
info "veritydevice='$veritydevice'"
info "overlays='$overlays'"
info "overlays_debug_mount='$overlays_debug_mount'"

# create_overlay <path>
#
# Create a writable overlay for a folder <path> inside the verity disk.
# The path must already exist in the verity disk for an overlay to be added.
#	$1: Path relative to the rootfs (ie '/var')
create_overlay() {
    local _folder=$1
    local _mounted_folder="$VERITY_MOUNT/$_folder"
    local _overlay_name=$(str_replace ${_mounted_folder} '/' '_')
    local _overlay_folder=$(mktemp -d --tmpdir=${OVERLAY_TMPFS} "${_overlay_name}.XXXXXXXXXX")
    local _working="${_overlay_folder}/working"
    local _upper="${_overlay_folder}/upper"
    
    info "Creating a R/W overlay for $_folder"
    [ -d "$_mounted_folder" ] || die "$_folder does not exist, cannot create overlay"
    
    [ ! -d "${_working}" ] || die "Name collision with ${_working}"
    [ ! -d "${_upper}" ] || die "Name collision with ${_upper}"
    
    mkdir -p "${_working}"
    mkdir -p "${_upper}"
    
    mount -t overlay overlay -o ${OVERLAY_MNT_OPTS},lowerdir="${_mounted_folder}",upperdir="${_upper}",workdir="${_working}" "${_mounted_folder}" || \
        die "Failed to mount overlay in ${_mounted_folder}"
}

# Mount the verity disk to $NEWROOT, create a dummy device at /dev/root to
# satisfy wait_for_dev
mount_root() {
    info "Mounting verity root"
    mkdir -p "${VERITY_MOUNT}"
    veritysetup verify --debug --verbose ${veritydisk} ${verityhashtree} $(cat ${verityroothash}) || \
        die "Failed to validate verity disk"
    veritysetup create --debug --verbose ${veritydevice} ${veritydisk} ${verityhashtree} $(cat ${verityroothash})
    
    mount -o ro,defaults "/dev/mapper/${veritydevice}" "${VERITY_MOUNT}" || \
        die "Failed to mount verity root"	
    
    if [ -n ${verity_overlays} ]; then
        # Create working directories for overlays
        mkdir -p "${OVERLAY_TMPFS}"
        mount -t tmpfs tmpfs -o ${OVERLAY_MNT_OPTS},size=20% "${OVERLAY_TMPFS}" || \
            die "Failed to create overlay tmpfs at ${OVERLAY_TMPFS}"
        
        for _folder in ${verity_overlays}; do
            create_overlay $_folder
        done
        
        if [ -n "${overlays_debug_mount}" ]; then
            info "Adding overlay debug mount to ${overlays_debug_mount}"
            mount --bind "${OVERLAY_TMPFS}" "$NEWROOT/${overlays_debug_mount}" || die
        fi
    else
        info "No verity RW overlays set, mounting fully read-only"
    fi

    # Remount the verity disk and any overlays into the destination root
    mount --rbind "${VERITY_MOUNT}" "${NEWROOT}"

    # Signal completion 
    ln -s /dev/null /dev/root
}

expand_persistent_dev() {
    local _dev=$1

    case "$_dev" in
        LABEL=*)
            _dev="/dev/disk/by-label/${_dev#LABEL=}"
            ;;
        UUID=*)
            _dev="${_dev#UUID=}"
            _dev="${_dev,,}"
            _dev="/dev/disk/by-uuid/${_dev}"
            ;;
        PARTUUID=*)
            _dev="${_dev#PARTUUID=}"
            _dev="${_dev,,}"
            _dev="/dev/disk/by-partuuid/${_dev}"
            ;;
        PARTLABEL=*)
            _dev="/dev/disk/by-partlabel/${_dev#PARTLABEL=}"
            ;;
    esac
    printf "%s" "$_dev"
}

if [ -n "$verityroot" -a -z "${verityroot%%verityroot:*}" ]; then
    veritydisk=$(expand_persistent_dev ${verityroot#verityroot:})
    verityhashtree=$(expand_persistent_dev $verityhashtree)
    verityroothash=$(expand_persistent_dev $verityroothash)
    mount_root
fi
