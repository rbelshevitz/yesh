#!/bin/sh
# Disposable GPT/PARTUUID fixture. Run as root on a Linux test host:
#   sudo sh /home/max/yesh-gpt-partuuid-fixture.sh
#
# Only a private image in /tmp is partitioned. No host disk is selected.

set -eu
umask 077

test "$(id -u)" -eq 0
test -x /sbin/sfdisk
test -x /sbin/losetup
test -x /sbin/blkid
test -x /sbin/mkfs.ext4
test -x /bin/mount
test -x /bin/umount
test -x /bin/findmnt
test -x /bin/lsblk

base=$(mktemp -d /tmp/yesh-gpt.XXXXXX)
image=$base/disk.img
mountpoint=$base/mnt
loop=
partition=

cleanup() {
    rc=$?
    if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
        umount -- "$mountpoint" || rc=1
    fi
    if test -n "$loop"; then
        losetup --detach "$loop" >/dev/null 2>&1 || rc=1
    fi
    case "$base" in
        /tmp/yesh-gpt.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$mountpoint"
truncate -s 96M "$image"
loop=$(/sbin/losetup --find --show --nooverlap --partscan "$image")
case "$loop" in
    /dev/loop[0-9]*) ;;
    *) echo "ERROR: losetup returned an unexpected device: $loop" >&2; exit 1 ;;
esac
/sbin/sfdisk "$loop" >/dev/null <<EOF
label: gpt
,64M,L
EOF

partition=$(/bin/lsblk -nrpo NAME,TYPE "$loop" |
    awk '$2 == "part" {print $1; exit}')
test -n "$partition"
case "$partition" in
    "$loop"p1) ;;
    *) echo "ERROR: unexpected partition path: $partition" >&2; exit 1 ;;
esac

/sbin/mkfs.ext4 -F "$partition" >/dev/null
partuuid=$(/sbin/blkid -s PARTUUID -o value "$partition")
uuid=$(/sbin/blkid -s UUID -o value "$partition")
test -n "$partuuid"
test -n "$uuid"
printf 'IDENTITY PARTUUID=%s UUID=%s DEVICE=%s\n' "$partuuid" "$uuid" "$partition"

/bin/mount -t ext4 -o rw "$partition" "$mountpoint"
echo 'GPT/PARTUUID fixture' >"$mountpoint/probe"
/bin/umount -- "$mountpoint"

/bin/mount -t ext4 -o ro,noload "$partition" "$mountpoint"
info=$(/bin/findmnt --kernel --mountpoint "$mountpoint"     --output TARGET,SOURCE,FSTYPE,UUID,PARTUUID,MAJ:MIN,VFS-OPTIONS,FS-OPTIONS --noheadings)
printf 'MOUNT %s\n' "$info"
case "$info" in
    *"$partition"*ext4*"$uuid"*"$partuuid"*ro*norecovery*) ;;
    *) echo "ERROR: mount identity/options were not confirmed: $info" >&2; exit 1 ;;
esac
test "$(cat "$mountpoint/probe")" = 'GPT/PARTUUID fixture'
if touch "$mountpoint/must-fail" 2>/dev/null; then
    echo 'ERROR: readonly GPT mount accepted a write' >&2
    exit 1
fi
echo 'gpt partition and PARTUUID fixture passed'

