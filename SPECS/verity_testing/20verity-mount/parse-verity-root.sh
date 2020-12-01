#!/bin/sh

# Verity roots are specified by root=verity:<device>

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh
info "Calculating Verity Root!"

# Make sure we have a verity root
[ -z "$veritydevice" ] && veritydevice=$(getarg rd.verityroot.device=)
[ -z "$veritymount" ] && veritymount=$(getarg root=)
[ -z "$verityhashtree"] && verityhashtree=$(getarg rd.verityroot.hashtree=)
[ -z "$verityroothash"] && verityroothash=$(getarg rd.verity.root.roothash=)

#Borrowed from dracut-functions.sh
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

veritydisk=$(expand_persistent_dev $veritydevice)
[ -n "$veritydisk" ]
info "Going to try to mount $veritydisk to $veritymount"

verityhashtree=$(expand_persistent_dev $verityhashtree)
verityroothash=$(expand_persistent_dev $verityroothash)

# Make sure we have all our components
[ -n "$veritydisk" ]
[ -n "$veritymount" ]
[ -n "$verityhashtree" ]
[ -n "$verityroothash" ]