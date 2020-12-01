#!/bin/sh

# Make sure we have dracut-lib loaded
type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

[ -z "$root" ] && root=$(getarg root=)

# Look for a parameter of the form: root=verityroot:<DEVICE_TYPE>=<DEVICE_ID>
#str_starts
if [ "${root%%:*}" = "verityroot" ] ; then
    verityroot=$root
fi

# Bail early if no 'verityroot' root is found
[ "${verityroot%%:*}" = "verityroot" ] || exit 0

# Get all other required parameters
[ -z "$verityhashtree" ] && verityhashtree=$(getarg rd.verityroot.hashtree=)
[ -z "$verityroothash" ] && verityroothash=$(getarg rd.verityroot.roothash=)
[ -z "$veritydevicename" ] && veritydevicename=$(getarg rd.verityroot.devicename=)
[ -n "$veritydevicename" ] || veritydevicename="verity_root"

[ -n "$verityhashtree" ] || die "verityroot requires rd.verityroot.hashtree="
[ -n "$verityroothash" ] || die "verityroot requires rd.verityroot.roothash="


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


veritydisk=$(expand_persistent_dev ${verityroot#verityroot:})
verityhashtree=$(expand_persistent_dev $verityhashtree)
verityroothash=$(expand_persistent_dev $verityroothash)

info "Going to try to mount $verityroot with $verityhashtree and $verityroothash"
rootok=1
unset root
root="${verityroot}"

# We still want to wait for /dev/root, but we won't actually mount anything there
#wait_for_dev "/dev/root"

info "waiting for ${veritydisk} ${verityhashtree} ${verityroothash}"

[ "${root%%:*}" = "verityroot" ] && \
    wait_for_dev "${veritydisk}"  && \
    wait_for_dev "${verityhashtree}" && \
    wait_for_dev "${verityroothash}" 

info "Done Waiting"
