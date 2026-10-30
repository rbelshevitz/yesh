#!/bin/sh
# Disposable recursive-bind fixture. Run as root:
#   sudo sh /home/max/yesh-rbind-fixture.sh
#
# Two private ext4 images are used. No host filesystem or fstab is changed.

set -eu
umask 077

test "$(id -u)" -eq 0
test -x /sbin/mkfs.ext4
test -x /bin/mount
test -x /bin/umount
test -x /bin/findmnt

base=$(mktemp -d /tmp/yesh-rbind.XXXXXX)
main_image=$base/main.img
nested_image=$base/nested.img
main_mount=$base/main
nested_mount=$main_mount/tree/nested
bind_mount=$base/bind
nested_loop=
main_loop=

cleanup() {
    rc=$?
    if findmnt --kernel --mountpoint "$bind_mount" >/dev/null 2>&1; then
        /bin/umount -R -- "$bind_mount" || rc=1
    fi
    if findmnt --kernel --mountpoint "$nested_mount" >/dev/null 2>&1; then
        /bin/umount -- "$nested_mount" || rc=1
    fi
    if findmnt --kernel --mountpoint "$main_mount" >/dev/null 2>&1; then
        /bin/umount -- "$main_mount" || rc=1
    fi
    case "$base" in
        /tmp/yesh-rbind.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$main_mount" "$bind_mount"
truncate -s 64M "$main_image"
truncate -s 32M "$nested_image"
/sbin/mkfs.ext4 -F "$main_image" >/dev/null
/sbin/mkfs.ext4 -F "$nested_image" >/dev/null

/bin/mount -t ext4 -o loop,rw "$main_image" "$main_mount"
mkdir -m 700 "$main_mount/tree" "$nested_mount"
/bin/mount -t ext4 -o loop,rw "$nested_image" "$nested_mount"
nested_loop=$(/bin/findmnt --kernel --mountpoint "$nested_mount"     --output SOURCE --noheadings)
case "$nested_loop" in
    /dev/loop*) ;;
    *) echo "ERROR: nested mount is not loop-backed: $nested_loop" >&2; exit 1 ;;
esac
echo 'nested rbind fixture' >"$nested_mount/marker"

/bin/mount --rbind "$main_mount/tree" "$bind_mount"
bind_info=$(/bin/findmnt --kernel --mountpoint "$bind_mount"     --output TARGET,SOURCE,FSTYPE,FSROOT --noheadings)
nested_info=$(/bin/findmnt --kernel --mountpoint "$bind_mount/nested"     --output TARGET,SOURCE,FSTYPE,FSROOT --noheadings)
printf 'RBIND %s\n' "$bind_info"
printf 'RBIND-NESTED %s\n' "$nested_info"
case "$nested_info" in
    *"$bind_mount/nested"*"$nested_loop"*ext4*) ;;
    *) echo "ERROR: rbind did not carry nested mount: $nested_info" >&2; exit 1 ;;
esac
test "$(cat "$bind_mount/nested/marker")" = 'nested rbind fixture'

/bin/umount -R -- "$bind_mount"
if findmnt --kernel --mountpoint "$bind_mount/nested" >/dev/null 2>&1; then
    echo 'ERROR: recursive unmount left nested bind mount' >&2
    exit 1
fi
echo 'recursive bind and nested-mount cleanup passed'

