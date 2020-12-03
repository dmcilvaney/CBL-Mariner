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
[ -z "$veritydevicename" ] && veritydevicename=$(getarg rd.verityroot.devicename=)
[ -n "$veritydevicename" ] || veritydevicename="verity_root"
[ -z "$verityhashtree" ] && verityhashtree=$(getarg rd.verityroot.hashtree=)
[ -z "$verityroothash" ] && verityroothash=$(getarg rd.verityroot.roothash=)

[ -z "$verityroothashsig" ] && verityroothashsig=$(getarg rd.verityroot.roothashsig=/path/to/file=)
[ -z "$verityerrorhandling" ] && verityerrorhandling=$(getarg rd.verityroot.verityerrorhandling=)
[ -z "$validateonboot" ] && validateonboot=$(getarg rd.verityroot.validateonboot=)
[ -z "$verityfecdata" ] && verityfecdata=$(getarg rd.verityroot.fecdata=)
[ -z "$verityfecroots" ] && verityfecroots=$(getarg rd.verityroot.fecroots=)
[ -z "$overlays" ] && verity_overlays=$(getarg rd.verityroot.overlays=)
[ -z "$overlays_debug_mount" ] && overlays_debug_mount=$(getarg rd.verityroot.overlays_debug_mount=)

# Check the required parameters are pressent
[ -n "$veritydevicename" ] || die "verityroot requires rd.verityroot.devicename="
[ -n "$verityhashtree" ] || die "verityroot requires rd.verityroot.hashtree="
[ -n "$verityroothash" ] || die "verityroot requires rd.verityroot.roothash="

# Validate the optional paramters
# Make sure we have either both or neither FEC arguments
[ -n "$verityfecdata" -a -z "$verityfecroots" ] && die "verityroot FEC requires both rd.verityroot.fecdata= and rd.verityroot.fecroots="
[ -z "$verityfecdata" -a -n "$verityfecroots" ] && die "verityroot FEC requires both rd.verityroot.fecdata= and rd.verityroot.fecroots="

if [ -n "$verityerrorhandling" ]; then 
    [ "$verityerrorhandling" == "ignore" -o \
    "$verityerrorhandling" == "restart" -o \
    "$verityerrorhandling" == "panic" ] || die "verityroot rd.verityroot.verityerrorhandling= must be one of [ignore,restart,panic]"
fi

if [ -n "$validateonboot" ]; then 
    [ "$validateonboot" == "true" -o \
    "$validateonboot" == "false" ] || die "verityroot rd.verityroot.validateonboot= must be one of [true,false]"
fi

info "verityroot='$verityroot'"
info "veritydevicename='$veritydevicename'"
info "verityhashtree='$verityhashtree'"
info "verityroothash='$verityroothash'"
info "verityerrorhandling='$verityerrorhandling'"
info "validateonboot='$validateonboot'"
info "verityfecdata='$verityfecdata'"
info "verityfecroots='$verityfecroots'"
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

    # Convert error handling options into arguments
    if [ "$verityerrorhandling" == "restart" ]; then
        errorarg="--restart-on-corruption"
    elif [ "$verityerrorhandling" == "panic" ]; then
        errorarg="--panic-on-corruption"
    elif [ "$verityerrorhandling" == "ignore" ]; then
        errorarg="--ignore-corruption"
    fi

    # Convert FEC options to argument
    if [ -n "$verityfecdata" -a -n "$verityfecroots" ]; then
        fecargs="--fec-device=${verityfecdata} --fec-roots=${verityfecroots}"
    fi

    # Convert root hash sigh to argument
    if [ -n "$verityroothashsig" ]; then
        roothashsigargs="--root-hash-signature=${verityroothashsig}"
    fi

    if [ "$validateonboot" == "true" ]; then
        # verify does not support error handling args, ommit
        veritysetup --debug --verbose ${roothashsigargs} ${fecargs} verify ${veritydisk} ${verityhashtree} $(cat ${verityroothash}) || \
            die "Failed to validate verity disk"
    fi
    veritysetup --debug --verbose ${roothashsigargs} ${errorarg} ${fecargs} open ${veritydisk} ${veritydevicename} ${verityhashtree} $(cat ${verityroothash}) || die "Failed to create verity root"
    
    mount -o ro,defaults "/dev/mapper/${veritydevicename}" "${VERITY_MOUNT}" || \
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
            mount --bind "${OVERLAY_TMPFS}" "$VERITY_MOUNT/${overlays_debug_mount}" || die
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
