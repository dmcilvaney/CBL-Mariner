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
    #Required:
        #rd.verityroot.devicename=desired_device_mapper_name
        #rd.verityroot.hashtree=/path/to/hashtree | <DEVICE_TYPE>=<DEVICE_ID>
        #rd.verityroot.roothash=/path/to/roothash

    #Optional
        #rd.verityroot.roothashsig=/path/to/file
        #rd.verityroot.verityerrorhandling=ignore|restart|panic
        #rd.verityroot.validateonboot=true/false
        #rd.verityroot.fecdata=/path/to/fecdata | <DEVICE_TYPE>=<DEVICE_ID>
        #rd.verityroot.fecroots=#
        #rd.verityroot.overlays="/path/to/overlay/directory /other/path"
        #rd.verityroot.overlays_debug_mount=/path/to/mount/debug/info
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
root="${verityroot}"

# We still want to wait for /dev/root, but we won't actually mount anything there
#wait_for_dev "/dev/root"

info "waiting for ${veritydisk} ${verityhashtree} ${verityroothash}"

[ "${root%%:*}" = "verityroot" ] && \
    wait_for_dev "${veritydisk}"  && \
    wait_for_dev "${verityhashtree}" && \
    wait_for_dev "${verityroothash}" 

info "Done Waiting"
