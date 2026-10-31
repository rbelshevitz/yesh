#!/bin/sh
# RWE B1: real yesh incremental rsync on a disposable mounted filesystem.
# Usage:
#   sudo sh /home/max/yesh-rsync-mounted-rwe.sh /home/max/yesh-rwe /home/max/rsync-incremental.yesh

set -eu
umask 077

yesh_bin=$1
example=$2
test -x "$yesh_bin"
test -r "$example"
test "$(id -u)" -eq 0
test -x /sbin/mkfs.ext4
test -x /bin/mount
test -x /bin/umount
test -x /bin/findmnt
test -x /usr/bin/rsync

base=$(mktemp -d /tmp/yesh-rsync-mounted-rwe.XXXXXX)
image=$base/backup.img
mountpoint=$base/mnt
source=$base/source
root=$mountpoint/snapshots
restore=$base/restore

cleanup() {
    rc=$?
    if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
        umount -- "$mountpoint" || rc=1
    fi
    case "$base" in
        /tmp/yesh-rsync-mounted-rwe.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$mountpoint" "$source" "$restore"
truncate -s 96M "$image"
/sbin/mkfs.ext4 -F "$image" >/dev/null
/bin/mount -t ext4 -o loop,rw "$image" "$mountpoint"
mkdir "$root"

printf '%s\n' 'version one' >"$source/changed.txt"
printf '%s\n' 'unchanged' >"$source/unchanged.txt"
mkdir "$source/subdir"
printf '%s\n' 'nested' >"$source/subdir/nested.txt"

test "$(/bin/findmnt --kernel --target "$root" --output TARGET --noheadings)" = "$mountpoint"
/bin/findmnt --kernel --target "$root" --output TARGET,SOURCE,FSTYPE,VFS-OPTIONS,FS-OPTIONS

run_yeshrsync() {
    YESH_RSYNC_SOURCE=$source/ \
    YESH_RSYNC_ROOT=$root \
    YESH_RSYNC_PREVIOUS=$1 \
    YESH_RSYNC_CURRENT=$2 \
    YESH_RSYNC_TEMPORARY=$3 \
    YESH_RSYNC_LOCK=$root/.rsync.lock \
    "$yesh_bin" "$example"
}

YESH_RSYNC_SOURCE=$source/ \
YESH_RSYNC_ROOT=$root \
YESH_RSYNC_PREVIOUS=$root/snapshot.0 \
YESH_RSYNC_CURRENT=$root/snapshot.0 \
YESH_RSYNC_TEMPORARY=$root/snapshot.0.tmp \
YESH_RSYNC_LOCK=$root/.rsync.lock \
"$yesh_bin" --plan "$example" >"$base/plan.out"
test ! -e "$root/snapshot.0"
test ! -e "$root/snapshot.0.tmp"

run_yeshrsync "$root/snapshot.0" "$root/snapshot.0" "$root/snapshot.0.tmp"
test "$(cat "$root/snapshot.0/changed.txt")" = 'version one'
test "$(cat "$root/snapshot.0/unchanged.txt")" = 'unchanged'
test ! -e "$root/snapshot.0.tmp"

printf '%s\n' 'version two, changed size' >"$source/changed.txt"
run_yeshrsync "$root/snapshot.0" "$root/snapshot.1" "$root/snapshot.1.tmp"
test "$(cat "$root/snapshot.0/changed.txt")" = 'version one'
test "$(cat "$root/snapshot.1/changed.txt")" = 'version two, changed size'
test "$(cat "$root/snapshot.1/unchanged.txt")" = 'unchanged'
test "$(stat -c %i "$root/snapshot.0/unchanged.txt")" = "$(stat -c %i "$root/snapshot.1/unchanged.txt")"
test ! -e "$root/snapshot.1.tmp"

/usr/bin/rsync -a -- "$root/snapshot.1/" "$restore/"
test "$(cat "$restore/changed.txt")" = 'version two, changed size'
test "$(cat "$restore/unchanged.txt")" = 'unchanged'
test "$(cat "$restore/subdir/nested.txt")" = 'nested'

mv -- "$source" "$base/source-missing"
if run_yeshrsync "$root/snapshot.1" "$root/snapshot.2" "$root/snapshot.2.tmp"; then
    echo 'ERROR: missing source accepted' >&2
    exit 1
fi
test -d "$root/snapshot.1"
test ! -e "$root/snapshot.2"
test ! -e "$root/snapshot.2.tmp"
test "$(cat "$root/snapshot.1/changed.txt")" = 'version two, changed size'

echo 'RWE mounted incremental rsync, plan, link-dest, restore, and missing-source negative passed'

