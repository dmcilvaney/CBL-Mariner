#!/bin/sh

# Make sure we h ave dracut-lib loaded
type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

info "Calculating Verity Root!"

# Make sure we have a verity root
[ -z "$veritydevice" ] && veritydevice=$(getarg rd.verityroot.device=)
[ -z "$veritymount" ] && veritymount=$(getarg root=)
[ -z "$verityhashtree"] && verityhashtree=$(getarg rd.verityroot.hashtree=)
[ -z "$verityroothash"] && verityroothash=$(getarg rd.verity.root.roothash=)

# Make sure the mount point is a dm device, then strip the path
[ -z "${veritymount##/dev/mapper/*}" ]
veritymount=${veritymount##/dev/mapper/}

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


info "Creating verity generator"
# Create a systemd service to create the verity device.
service_dir="$1"
systemd_verity_device="$(dev_unit_name ${veritydisk})"
systemd_verity_mount="$(dev_unit_name /dev/mapper/${veritymount})"
servicefile="${service_dir}/verity_setup.service"
info "Placing service into $servicefile"
info "Binding to ${systemd_verity_device}.device ${systemd_verity_mount}.device"
{
    echo "[Unit]"
    echo "Description=verity_setup: Mount a verity root to ${veritymount}"
    echo "Before=sysinit.target"
    echo "After=${systemd_verity_device}.device"
    echo "DefaultDependencies=no"
    echo "BindsTo=${systemd_verity_device}.device" # ${systemd_verity_mount}.device"

    echo "[Service]"
    echo "Type=oneshot"
    echo "RemainAfterExit=yes"
    echo "StandardOutput=journal+console"
    # veritysetup does not return an error code if the device is corrupt, explicitly verify it first
    #echo "ExecStartPre=/sbin/veritysetup verify --debug --verbose ${veritydisk} ${verityhashtree} $(cat ${verityroothash})"
    echo "ExecStart=/sbin/veritysetup create --debug --verbose ${veritymount} ${veritydisk} ${verityhashtree} $(cat ${verityroothash})"
    echo "ExecStop=/sbin/veritysetup remove --debug --verbose ${veritymount}"

    echo "[Install]"
    echo "RequiredBy=sysinit.target"

} > $servicefile

# Enforce the "WantedBy" requirement
requires_dir="${service_dir}/sysinit.target.requires/"
requires_target="${requires_dir}/verity-setup.service"
info "linking ${requires_target} to ${servicefile}"
mkdir -p ${requires_dir}
ln -s ${servicefile} ${requires_dir}

exit 0

# # Wait on the 
# mkdir -p ${servicefile}.wants


# _name=$(dev_unit_name "$1")
# if ! [ -L "$GENERATOR_DIR"/initrd.target.wants/${_name}.device ]; then
#     [ -d "$GENERATOR_DIR"/initrd.target.wants ] || mkdir -p "$GENERATOR_DIR"/initrd.target.wants
#     ln -s ../${_name}.device "$GENERATOR_DIR"/initrd.target.wants/${_name}.device
# fi

#     inst "$moddir/squash-mnt-clear.service" "$systemdsystemunitdir/squash-mnt-clear.service"
#     mkdir -p "$initdir$systemdsystemunitdir/initrd-switch-root.target.wants"
#     ln_r "$systemdsystemunitdir/squash-mnt-clear.service" "$systemdsystemunitdir/initrd-switch-root.target.wants/squash-mnt-clear.service"

# mkdir -p /var/tmp/dracut.dg4iVi/initramfs/usr/lib/systemd/system/initrd-switch-root.target.wants
# /usr/lib/systemd/system/squash-mnt-clear.service /usr/lib/systemd/system/initrd-switch-root.target.wants/squash-mnt-clear.service

#wait_for_dev $veritymount