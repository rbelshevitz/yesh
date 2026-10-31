#!/usr/bin/env bash
set -euo pipefail

# Disposable Linux acceptance scenario for rsnapshot-apply.yesh.
# The caller must provide an already-built yesh binary and the example script.
# This script owns only a temporary image, loop device, mount and test data.
# It never uses /etc/rsnapshot.conf and never touches a real backup root.

yesh_bin=${YESH_BIN:-./yesh}
yesh_script=${YESH_SCRIPT:-examples/rsnapshot-apply.yesh}
work=${YESH_RSNAPSHOT_WORK:-}
created_work=0
loop_device=
mounted=0

die() {
    echo "yesh-rsnapshot-loopback: $*" >&2
    exit 1
}

cleanup() {
    status=$?
    trap - EXIT INT TERM
    if [ "$mounted" -eq 1 ]; then
        if ! umount "$mountpoint"; then
            echo "yesh-rsnapshot-loopback: could not unmount $mountpoint; preserving $work" >&2
            exit 1
        fi
        mounted=0
    fi
    if [ -n "$loop_device" ]; then
        losetup --detach "$loop_device" || status=1
        loop_device=
    fi
    if [ "$created_work" -eq 1 ]; then
        rm -rf -- "$work"
    fi
    exit "$status"
}
trap cleanup EXIT INT TERM

[ "$(id -u)" -eq 0 ] || die "run explicitly as root (for example: sudo $0)"
for command in losetup mkfs.ext4 mount umount findmnt rsnapshot rsync cp rm stat cmp tee grep test; do
    command -v "$command" >/dev/null || die "missing command: $command"
done
[ -x "$yesh_bin" ] || die "yesh binary is not executable: $yesh_bin"
[ -f "$yesh_script" ] || die "yesh script is missing: $yesh_script"

if [ -z "$work" ]; then
    work=$(mktemp -d /tmp/yesh-rsnapshot-loop.XXXXXX)
    created_work=1
else
    [ ! -e "$work" ] || die "explicit work directory already exists: $work"
    mkdir -m 0700 -- "$work"
fi

image=$work/snapshot-root.ext4
mountpoint=$work/mount
source=$work/source
snapshot_root=$mountpoint/snapshots
restore=$work/restore
config=$work/rsnapshot.conf
bad_config=$work/rsnapshot-bad.conf
lockfile=$work/rsnapshot.pid
report=$work/rsnapshot-test.log
confirm=$work/approved
published=$snapshot_root/daily.0
bad_root=$mountpoint/bad-snapshots
bad_published=$bad_root/daily.0

mkdir -m 0700 -- "$mountpoint" "$source" "$restore"
truncate -s 256M "$image"
mkfs.ext4 -F -q "$image"
loop_device=$(losetup --find --show --nooverlap "$image")
mount -o nosuid,nodev,noexec "$loop_device" "$mountpoint"
mounted=1
mkdir -m 0700 -- "$snapshot_root"
mkdir -m 0700 -- "$bad_root"

printf 'version-one\n' > "$source/versioned.txt"
printf 'unchanged\n' > "$source/unchanged.txt"
mount_source=$(findmnt -no SOURCE --target "$mountpoint")

printf 'config_version\t1.2\nsnapshot_root\t%s/\nno_create_root\t1\nretain\tdaily\t2\nloglevel\t2\nlockfile\t%s\ncmd_rsync\t%s\ncmd_cp\t%s\ncmd_rm\t%s\nbackup\t%s/./\tlocal/\n' \
    "$snapshot_root" "$lockfile" "$(command -v rsync)" "$(command -v cp)" "$(command -v rm)" "$source" > "$config"
touch "$confirm"

common_env=(
    YESH_RSNAPSHOT_CONFIG="$config"
    YESH_RSNAPSHOT_LEVEL=daily
    YESH_RSNAPSHOT_ROOT="$snapshot_root"
    YESH_RSNAPSHOT_SOURCE="$mount_source"
    YESH_RSNAPSHOT_LOCK="$lockfile"
    YESH_RSNAPSHOT_REPORT="$report"
    YESH_RSNAPSHOT_PUBLISHED="$published"
    YESH_RSNAPSHOT_CONFIRM="$confirm"
)

env "${common_env[@]}" "$yesh_bin" --plan "$yesh_script" > "$work/plan.out"
[ ! -e "$published" ] || die "--plan created a snapshot"
grep -F 'rsnapshot' "$work/plan.out" >/dev/null || die "--plan omitted rsnapshot argv"
grep -F 'configtest' "$work/plan.out" >/dev/null || die "--plan omitted configtest"
grep -F -- '-t' "$work/plan.out" >/dev/null || die "--plan omitted test mode"

cp -- "$config" "$bad_config"
printf 'unknown_option\t1\n' >> "$bad_config"
set +e
env "${common_env[@]}" YESH_RSNAPSHOT_CONFIG="$bad_config" YESH_RSNAPSHOT_ROOT="$bad_root" YESH_RSNAPSHOT_PUBLISHED="$bad_published" "$yesh_bin" "$yesh_script" > "$work/bad-config.out" 2>&1
bad_status=$?
set -e
[ "$bad_status" -ne 0 ] || die "invalid rsnapshot configuration unexpectedly succeeded"
[ ! -e "$bad_published" ] || die "invalid configuration created a snapshot"

rm -- "$confirm"
set +e
env "${common_env[@]}" "$yesh_bin" "$yesh_script" > "$work/missing-confirm.out" 2>&1
missing_confirm_status=$?
set -e
[ "$missing_confirm_status" -ne 0 ] || die "missing confirmation unexpectedly succeeded"
[ ! -e "$published" ] || die "missing confirmation created a snapshot"
touch -- "$confirm"

env "${common_env[@]}" "$yesh_bin" "$yesh_script" > "$work/run1.out"
[ -s "$report" ] || die "rsnapshot test report was not captured"
grep -F 'rsnapshot' "$report" >/dev/null || die "rsnapshot test report was empty"
cmp "$snapshot_root/daily.0/local/versioned.txt" <(printf 'version-one\n')

printf 'version-two-changed\n' > "$source/versioned.txt"
env "${common_env[@]}" "$yesh_bin" "$yesh_script" > "$work/run2.out"
cmp "$snapshot_root/daily.0/local/versioned.txt" <(printf 'version-two-changed\n')
cmp "$snapshot_root/daily.1/local/versioned.txt" <(printf 'version-one\n')

first_inode=$(stat -c '%i' "$snapshot_root/daily.0/local/unchanged.txt")
second_inode=$(stat -c '%i' "$snapshot_root/daily.1/local/unchanged.txt")
[ "$first_inode" = "$second_inode" ] || die "unchanged file did not retain its inode"
cp -- "$snapshot_root/daily.1/local/versioned.txt" "$restore/versioned.txt"
cmp "$restore/versioned.txt" <(printf 'version-one\n')
[ ! -e "$lockfile" ] || die "rsnapshot lockfile remains after the run"

touch -- "$lockfile"
set +e
env "${common_env[@]}" YESH_RSNAPSHOT_PUBLISHED="$snapshot_root/daily.2" "$yesh_bin" "$yesh_script" > "$work/held-lock.out" 2>&1
held_lock_status=$?
set -e
[ "$held_lock_status" -ne 0 ] || die "held rsnapshot lock unexpectedly succeeded"
[ ! -e "$snapshot_root/daily.2" ] || die "held lock published a snapshot"
rm -- "$lockfile"

echo "yesh-rsnapshot-loopback: PASS"
echo "image: $image"
echo "mount source: $mount_source"
