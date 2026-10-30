#!/bin/sh
# RWE for yesh retention quarantine.
# Usage:
#   sh /home/max/yesh-quarantine-rwe.sh /home/max/yesh-rwe /home/max/rsync-retention-quarantine.yesh

set -eu
umask 077

yesh_bin=$1
example=$2
test -x "$yesh_bin"
test -r "$example"

base=$(mktemp -d /tmp/yesh-quarantine-rwe.XXXXXX)
root=$base/snapshots
quarantine=$root/quarantine
lock=$root/.retention.lock

cleanup() {
    rc=$?
    case "$base" in
        /tmp/yesh-quarantine-rwe.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$root" "$quarantine"
for name in snapshot.0 snapshot.1 snapshot.2; do
    mkdir "$root/$name"
    printf '%s\n' "$name" >"$root/$name/marker"
done

export YESH_RSYNC_ROOT=$root
export YESH_RSYNC_KEEP=2
export YESH_RSYNC_QUARANTINE=$quarantine
export YESH_RSYNC_LOCK=$lock

"$yesh_bin" --plan "$example" >"$base/plan.out"
test -d "$root/snapshot.0"
test ! -e "$quarantine/snapshot.0"

"$yesh_bin" "$example"
test ! -e "$root/snapshot.0"
test "$(cat "$quarantine/snapshot.0/marker")" = snapshot.0
test -d "$root/snapshot.1"
test -d "$root/snapshot.2"

"$yesh_bin" "$example"
test ! -e "$quarantine/snapshot.1"
test ! -e "$quarantine/snapshot.2"
test -d "$root/snapshot.1"
test -d "$root/snapshot.2"

echo 'RWE yesh quarantine plan, locked rename, and safe rerun passed'

