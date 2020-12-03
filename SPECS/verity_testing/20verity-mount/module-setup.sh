#!/bin/bash

check() {
    # Only include if requested by the dracut configuration files
    return 0
}

depends() {
    echo systemd dm
}

cmdline() {
    echo "rd.verityroot.mount=NAME_HERE rd.verityroot.device=DEVICE_HERE rd.verityroot.hashtree=HASH_FILE_HERE rd.verity.root.roothash=ROOT_HASH_FILE_HERE"
    echo "rd.verityroot.overlay"
}

install() {
    inst "veritysetup"
    inst "mktemp"
    inst "tail"
    inst "less"
    inst "nano"
    #inst_hook cmdline 20 "$moddir/parse-verity-root.sh"
    inst_hook cmdline 20 "$moddir/verity_parse_v2.sh"
    #inst_hook pre-pivot 99 "$moddir/add-verity-overlays.sh"
    inst_hook pre-mount 10 "$moddir/verity_mount_v2.sh"
    #inst_script "$moddir/verity-root-generator.sh" "$systemdutildir/system-generators/dracut-verity-root-generator"
    dracut_need_initqueue

    #echo "mkdir -p $initdir$systemdsystemunitdir/initrd-switch-root.target.wants"
    #echo "$systemdsystemunitdir/squash-mnt-clear.service $systemdsystemunitdir/initrd-switch-root.target.wants/squash-mnt-clear.service"
}