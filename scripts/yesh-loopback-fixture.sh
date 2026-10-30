#!/bin/sh
# Run on a disposable Linux host as an unprivileged user:
#   sudo sh /home/max/yesh-loopback-fixture.sh
#
# The fixture uses only a private /tmp tree. It does not read or modify
# /etc/fstab, real disks, or system services.

set -eu
umask 077

base=$(mktemp -d /tmp/yesh-loop.XXXXXX)
image=$base/fs.img
mountpoint=$base/mnt
bindpoint=$base/bind
holder=

cleanup() {
    rc=$?
    if test -n "$holder"; then
        kill "$holder" 2>/dev/null || true
        wait "$holder" 2>/dev/null || true
    fi
    if findmnt --kernel --mountpoint "$bindpoint" >/dev/null 2>&1; then
        umount -- "$bindpoint" || rc=1
    fi
    if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
        umount -- "$mountpoint" || rc=1
    fi
    case "$base" in
        /tmp/yesh-loop.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

test "$(id -u)" -eq 0
test -x /sbin/mkfs.ext4
test -x /bin/mount
test -x /bin/umount
test -x /bin/findmnt

mkdir -m 700 "$mountpoint"
mkdir -m 700 "$bindpoint"
truncate -s 64M "$image"
/sbin/mkfs.ext4 -F "$image" >/dev/null

/bin/mount -t ext4 -o loop,rw "$image" "$mountpoint"
rw_options=$(/bin/findmnt --kernel --mountpoint "$mountpoint" \
    --output VFS-OPTIONS,FS-OPTIONS --noheadings)
case "$rw_options" in
    *rw*) ;;
    *) echo "ERROR: rw mount options are not effective: $rw_options" >&2; exit 1 ;;
esac
echo 'loopback fixture' >"$mountpoint/probe"
mkdir -m 700 "$mountpoint/tree"
echo 'bind fixture' >"$mountpoint/tree/marker"
/bin/mount --bind "$mountpoint/tree" "$bindpoint"
bind_info=$(/bin/findmnt --kernel --mountpoint "$bindpoint" \
    --output TARGET,SOURCE,FSTYPE,FSROOT --noheadings)
printf 'BIND %s\n' "$bind_info"
case "$bind_info" in
    *"$bindpoint"*)
        case "$bind_info" in
            */tree*) ;;
            *) echo "ERROR: bind mount did not expose subtree fsroot: $bind_info" >&2; exit 1 ;;
        esac
        ;;
    *) echo "ERROR: bind mount did not expose a subtree fsroot: $bind_info" >&2; exit 1 ;;
esac
test "$(cat "$bindpoint/marker")" = 'bind fixture'
/bin/umount -- "$bindpoint"
sync
/bin/umount -- "$mountpoint"

/bin/mount -t ext4 -o loop,ro,noload "$image" "$mountpoint"
ro_options=$(/bin/findmnt --kernel --mountpoint "$mountpoint" \
    --output VFS-OPTIONS,FS-OPTIONS --noheadings)
case "$ro_options" in
    *ro*) ;;
    *) echo "ERROR: readonly mount options are not effective: $ro_options" >&2; exit 1 ;;
esac
case "$ro_options" in
    *norecovery*) ;;
    *) echo "ERROR: ext4 readonly mount lacks norecovery: $ro_options" >&2; exit 1 ;;
esac
/bin/findmnt --kernel --mountpoint "$mountpoint" \
    --output TARGET,SOURCE,FSTYPE,VFS-OPTIONS,FS-OPTIONS

if touch "$mountpoint/must-fail" 2>/dev/null; then
    echo 'ERROR: readonly mount accepted a write' >&2
    exit 1
fi
test "$(cat "$mountpoint/probe")" = 'loopback fixture'
/bin/umount -- "$mountpoint"

if /bin/mount -t vfat -o loop "$image" "$mountpoint" 2>/dev/null; then
    echo 'ERROR: ext4 image mounted as vfat' >&2
    exit 1
fi
if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
    echo 'ERROR: failed wrong-type mount left a mount behind' >&2
    exit 1
fi

bad_image=$base/not-a-filesystem.img
truncate -s 1M "$bad_image"
if /bin/mount -t ext4 -o loop "$bad_image" "$mountpoint" 2>/dev/null; then
    echo 'ERROR: invalid image mounted as ext4' >&2
    exit 1
fi
if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
    echo 'ERROR: failed invalid-image mount left a mount behind' >&2
    exit 1
fi

/bin/mount -t ext4 -o loop,rw "$image" "$mountpoint"
sh -c 'cd "$1"; exec sleep 30' sh "$mountpoint" &
holder=$!
busy_umount=0
if /bin/umount -- "$mountpoint" 2>/dev/null; then
    busy_umount=1
fi
kill "$holder" 2>/dev/null || true
wait "$holder" 2>/dev/null || true
holder=
if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
    /bin/umount -- "$mountpoint"
fi
if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
    echo 'ERROR: mount remained after holder cleanup' >&2
    exit 1
fi
if test "$busy_umount" -eq 1; then
    echo 'busy mountpoint detached on this kernel; holder cleanup verified'
else
    echo 'busy mountpoint rejected until holder cleanup'
fi
echo 'loopback readonly fixture passed'
