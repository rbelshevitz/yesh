#!/bin/sh
# RWE fixture: real yesh + real disposable mounted ext4 target.
# Usage:
#   sudo sh /home/max/yesh-mounted-backup-rwe.sh /home/max/yesh /home/max/backup-mounted-rwe.yesh
#
# The image, temporary fstab, source and target live under /tmp only.

set -eu
umask 077

yesh_bin=$1
example=$2
test -x "$yesh_bin"
test -r "$example"
test "$(id -u)" -eq 0
test -x /sbin/mkfs.ext4
test -x /sbin/blkid
test -x /bin/mount
test -x /bin/umount
test -x /bin/findmnt
test -x /usr/bin/rsync

base=$(mktemp -d /tmp/yesh-mounted-rwe.XXXXXX)
image=$base/backup.img
mountpoint=$base/mnt
source=$base/source
target=$mountpoint/backup

cleanup() {
    rc=$?
    if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
        umount -- "$mountpoint" || rc=1
    fi
    case "$base" in
        /tmp/yesh-mounted-rwe.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$mountpoint" "$source"
truncate -s 64M "$image"
/sbin/mkfs.ext4 -F "$image" >/dev/null
uuid=$(/sbin/blkid -s UUID -o value "$image")
/bin/mount -t ext4 -o loop,rw "$image" "$mountpoint"
mkdir "$target"
printf '%s\n' 'RWE mounted backup' >"$source/data.txt"

export YESH_RWE_MOUNTPOINT=$mountpoint
export YESH_RWE_SOURCE=UUID=$uuid
export YESH_RWE_SOURCE_DIR=$source/
export YESH_RWE_TARGET=$target/

"$yesh_bin" --plan "$example" >"$base/plan.out"
test ! -e "$target/data.txt"
grep -F 'findmnt' "$base/plan.out" >/dev/null

"$yesh_bin" "$example" >"$base/run.out"
test "$(cat "$target/data.txt")" = 'RWE mounted backup'

export YESH_RWE_SOURCE=UUID=00000000-0000-0000-0000-000000000000
if "$yesh_bin" "$example" >"$base/negative.out" 2>&1; then
    echo 'ERROR: wrong UUID accepted' >&2
    exit 1
fi
test "$(cat "$target/data.txt")" = 'RWE mounted backup'

export YESH_RWE_SOURCE=UUID=$uuid
umount -- "$mountpoint"
/bin/mount -t ext4 -o loop,ro,noload "$image" "$mountpoint"
if "$yesh_bin" "$example" >"$base/readonly.out" 2>&1; then
    echo 'ERROR: readonly target accepted writable backup' >&2
    exit 1
fi
test "$(cat "$target/data.txt")" = 'RWE mounted backup'

umount -- "$mountpoint"
if "$yesh_bin" "$example" >"$base/unmounted.out" 2>&1; then
    echo 'ERROR: unmounted target accepted backup' >&2
    exit 1
fi
test ! -e "$target/new-file.txt"

/bin/mount -t ext4 -o loop,rw "$image" "$mountpoint"
dd if=/dev/zero of="$mountpoint/filler" bs=1M count=45 conv=fsync >/dev/null 2>&1
if "$yesh_bin" "$example" >"$base/low-space.out" 2>&1; then
    echo 'ERROR: low-space target accepted backup' >&2
    exit 1
fi
test "$(cat "$target/data.txt")" = 'RWE mounted backup'

echo 'RWE mounted-backup positive, plan, wrong-UUID, readonly, unmounted, and low-space negatives passed'

