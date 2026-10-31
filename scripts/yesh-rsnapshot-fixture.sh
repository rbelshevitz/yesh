#!/bin/sh
# Run on a disposable Linux host as an unprivileged user:
#   sh scripts/yesh-rsnapshot-fixture.sh /tmp/yesh /tmp/rsnapshot-apply.yesh
#
# The fixture uses only a private /tmp tree. It verifies yesh's guarded
# preflight, two real rsnapshot rotations, restore, hard-link reuse, and
# cleanup. It does not change /etc, mounts, or system services.

set -eu

yesh_bin=$1
wrapper=$2
fixture=$(mktemp -d /tmp/yesh-rsnapshot.XXXXXX)
root=$fixture/snapshots
source=$fixture/source
restore=$fixture/restore
config=$fixture/rsnapshot.conf
confirm=$fixture/confirm
report=$fixture/rsnapshot-test.log
mount_source=$(findmnt -no SOURCE --target "$fixture")

cleanup() {
    rm -rf -- "$fixture"
}
trap cleanup EXIT INT TERM

test -x "$yesh_bin"
test -r "$wrapper"
mkdir -m 700 "$root" "$source" "$restore"

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
lockfile	$fixture/rsnapshot.pid
backup	$source/./	local/
EOF

printf '%s\n' 'version one' >"$source/hello.txt"
printf '%s\n' 'unchanged' >"$source/unchanged.txt"

YESH_RSNAPSHOT_CONFIG=$config \
YESH_RSNAPSHOT_LEVEL=daily \
YESH_RSNAPSHOT_ROOT=$root \
YESH_RSNAPSHOT_SOURCE=$mount_source \
YESH_RSNAPSHOT_CONFIRM=$confirm \
YESH_RSNAPSHOT_REPORT=$report \
"$yesh_bin" --plan "$wrapper" >"$fixture/plan.out"
test ! -e "$root/daily.0"
test ! -e "$confirm"

touch "$confirm"
run_snapshot() {
    YESH_RSNAPSHOT_CONFIG=$config \
    YESH_RSNAPSHOT_LEVEL=daily \
    YESH_RSNAPSHOT_ROOT=$root \
    YESH_RSNAPSHOT_SOURCE=$mount_source \
    YESH_RSNAPSHOT_CONFIRM=$confirm \
    YESH_RSNAPSHOT_REPORT=$report \
    "$yesh_bin" "$wrapper"
}

run_snapshot
test -s "$report"
printf '%s\n' 'version two changed' >"$source/hello.txt"
run_snapshot

test "$(cat "$root/daily.0/local/hello.txt")" = 'version two changed'
test "$(cat "$root/daily.1/local/hello.txt")" = 'version one'
test "$(cat "$root/daily.0/local/unchanged.txt")" = 'unchanged'
test "$(cat "$root/daily.1/local/unchanged.txt")" = 'unchanged'
test "$(stat -c %i "$root/daily.0/local/unchanged.txt")" = "$(stat -c %i "$root/daily.1/local/unchanged.txt")"
rsync -a "$root/daily.1/local/" "$restore/"
test "$(sha256sum "$restore/hello.txt" | cut -d ' ' -f 1)" = "$(sha256sum "$root/daily.1/local/hello.txt" | cut -d ' ' -f 1)"
test ! -e "$root/daily.2"
test ! -e "$fixture/rsnapshot.pid"

echo 'remote rsnapshot fixture checks passed'
