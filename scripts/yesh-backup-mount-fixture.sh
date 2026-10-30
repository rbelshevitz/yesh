#!/bin/sh
# Disposable mounted-backup/restore fixture. Run as root:
#   sudo sh /home/max/yesh-backup-mount-fixture.sh
#
# Uses one private ext4 image in /tmp. It does not touch /etc/fstab or real disks.

set -eu
umask 077

test "$(id -u)" -eq 0
test -x /sbin/mkfs.ext4
test -x /bin/mount
test -x /bin/umount
test -x /bin/findmnt
test -x /usr/bin/rsync

base=$(mktemp -d /tmp/yesh-backup-mount.XXXXXX)
image=$base/backup.img
mountpoint=$base/mnt
source=$base/source
restore=$base/restore
loop_mount=

cleanup() {
    rc=$?
    if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
        umount -- "$mountpoint" || rc=1
    fi
    case "$base" in
        /tmp/yesh-backup-mount.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$mountpoint" "$source" "$restore"
truncate -s 64M "$image"
/sbin/mkfs.ext4 -F "$image" >/dev/null

printf '%s
' 'backup version one' >"$source/data.txt"
printf '%s
' 'unchanged' >"$source/unchanged.txt"
/bin/mount -t ext4 -o loop,rw "$image" "$mountpoint"
loop_mount=$(/bin/findmnt --kernel --mountpoint "$mountpoint" --output SOURCE --noheadings)
case "$loop_mount" in
    /dev/loop*) ;;
    *) echo "ERROR: backup target is not loop-backed: $loop_mount" >&2; exit 1 ;;
esac
options=$(/bin/findmnt --kernel --mountpoint "$mountpoint"     --output VFS-OPTIONS,FS-OPTIONS --noheadings)
case "$options" in
    *rw*) ;;
    *) echo "ERROR: backup target is not writable: $options" >&2; exit 1 ;;
esac

mkdir "$mountpoint/snapshot.0.tmp"
/usr/bin/rsync -a -- "$source/" "$mountpoint/snapshot.0.tmp/"
mv -- "$mountpoint/snapshot.0.tmp" "$mountpoint/snapshot.0"
test ! -e "$mountpoint/snapshot.0.tmp"
test "$(cat "$mountpoint/snapshot.0/data.txt")" = 'backup version one'
sync
umount -- "$mountpoint"

# The published snapshot is now only read back from the mounted backup image.
/bin/mount -t ext4 -o loop,ro,noload "$image" "$mountpoint"
ro_options=$(/bin/findmnt --kernel --mountpoint "$mountpoint"     --output VFS-OPTIONS,FS-OPTIONS --noheadings)
case "$ro_options" in
    *ro*norecovery*) ;;
    *) echo "ERROR: backup remount is not readonly,norecovery: $ro_options" >&2; exit 1 ;;
esac
/usr/bin/rsync -a -- "$mountpoint/snapshot.0/" "$restore/"
test "$(cat "$restore/data.txt")" = 'backup version one'
test "$(cat "$restore/unchanged.txt")" = 'unchanged'
if touch "$mountpoint/snapshot.0/must-fail" 2>/dev/null; then
    echo 'ERROR: readonly backup mount accepted a write' >&2
    exit 1
fi

echo "backup mount SOURCE=$loop_mount"
echo "backup mount restore verified"

