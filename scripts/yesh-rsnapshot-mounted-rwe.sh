#!/bin/sh
# RWE B2: mounted rsnapshot rotation and same-filesystem quarantine.
# Usage:
#   sudo sh /home/max/yesh-rsnapshot-mounted-rwe.sh /home/max/yesh-rwe /home/max/rsnapshot-apply.yesh

set -eu
umask 077

yesh_bin=$1
wrapper=$2
test -x "$yesh_bin"
test -r "$wrapper"
test "$(id -u)" -eq 0
test -x /sbin/mkfs.ext4
test -x /bin/mount
test -x /bin/umount
test -x /bin/findmnt
test -x /usr/bin/rsnapshot
test -x /usr/bin/rsync
test -x /usr/bin/flock

base=$(mktemp -d /tmp/yesh-rsnapshot-mounted-rwe.XXXXXX)
image=$base/backup.img
mountpoint=$base/mnt
source=$base/source
restore=$base/restore
root=$mountpoint/snapshots
config=$base/rsnapshot.conf
confirm=$base/confirm
phase=setup

cleanup() {
    rc=$?
    if test "$rc" -ne 0; then
        echo "ERROR: RWE phase=$phase exit=$rc" >&2
    fi
    if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
        umount -- "$mountpoint" || rc=1
    fi
    case "$base" in
        /tmp/yesh-rsnapshot-mounted-rwe.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$mountpoint" "$source" "$restore"
truncate -s 128M "$image"
/sbin/mkfs.ext4 -F "$image" >/dev/null
/bin/mount -t ext4 -o loop,rw "$image" "$mountpoint"
mkdir "$root"
mount_source=$(/bin/findmnt --kernel --target "$root" --output SOURCE --noheadings)

cat >"$config" <<EOF
config_version	1.2
snapshot_root	$root/
no_create_root	1
cmd_rsync	/usr/bin/rsync
cmd_cp	/bin/cp
cmd_rm	/bin/rm
retain	daily	2
verbose	1
loglevel	3
lockfile	$root/rsnapshot.pid
backup	$source/./	local/
EOF
printf '%s\n' 'rsnapshot version one' >"$source/data.txt"
touch "$confirm"

run_apply() {
    YESH_RSNAPSHOT_CONFIG=$config \
    YESH_RSNAPSHOT_LEVEL=daily \
    YESH_RSNAPSHOT_ROOT=$root \
    YESH_RSNAPSHOT_SOURCE=$mount_source \
    YESH_RSNAPSHOT_CONFIRM=$confirm \
    "$yesh_bin" "$wrapper"
}

YESH_RSNAPSHOT_CONFIG=$config \
YESH_RSNAPSHOT_LEVEL=daily \
YESH_RSNAPSHOT_ROOT=$root \
YESH_RSNAPSHOT_SOURCE=$mount_source \
YESH_RSNAPSHOT_CONFIRM=$confirm \
"$yesh_bin" --plan "$wrapper" >"$base/plan.out"
test ! -e "$root/daily.0"

run_apply
phase=second-rotation
printf '%s\n' 'rsnapshot version two' >"$source/data.txt"
if ! run_apply >"$base/second-rotation.out" 2>&1; then
    cat "$base/second-rotation.out" >&2
    exit 1
fi
cat "$base/second-rotation.out"
phase=verify-second
printf 'daily.0: '
cat "$root/daily.0/local/data.txt"
printf 'daily.1: '
cat "$root/daily.1/local/data.txt"
test "$(cat "$root/daily.0/local/data.txt")" = 'rsnapshot version two'
test "$(cat "$root/daily.1/local/data.txt")" = 'rsnapshot version one'

quarantine=$root/quarantine
phase=quarantine
mkdir "$quarantine"
exec 9>"$root/.quarantine.lock"
/usr/bin/flock -n 9
candidate=$root/daily.1
test -d "$candidate"
test ! -L "$candidate"
test "$(stat -c %d "$candidate")" = "$(stat -c %d "$quarantine")"
mv -- "$candidate" "$quarantine/daily.1"
test ! -e "$candidate"
test -d "$quarantine/daily.1"

/usr/bin/rsync -a -- "$quarantine/daily.1/local/" "$restore/"
phase=restore
test "$(cat "$restore/data.txt")" = 'rsnapshot version one'
test -d "$root/daily.0"
test ! -e "$root/daily.2"
exec 9>&-
echo 'RWE mounted rsnapshot rotation, plan, quarantine, and restore passed'

