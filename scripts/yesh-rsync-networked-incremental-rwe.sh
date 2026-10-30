#!/bin/sh
set -eu
umask 077

yesh=${1:-}
example=${2:-}
if test -z "$yesh" || test -z "$example"; then
    echo "usage: $0 /path/to/yesh /path/to/rsync-networked-incremental-rwe.yesh" >&2
    exit 2
fi
test -x "$yesh"
test -r "$example"

base=$(mktemp -d /tmp/yesh-rsync-networked-incremental-rwe.XXXXXX)
trap 'rm -rf -- "$base"' EXIT INT TERM HUP
root=$base/snapshots
mkdir "$root"

export YESH_RSYNC_DNS_NAME=192.168.111.113
export YESH_RSYNC_ROUTE_TARGET=192.168.111.113
export YESH_RSYNC_HOST=max@192.168.111.113
export YESH_RSYNC_REMOTE_PATH=/tmp/yesh-test
export YESH_RSYNC_SOURCE=max@192.168.111.113:/tmp/yesh-test/
export YESH_RSYNC_ROOT=$root
export YESH_RSYNC_CURRENT=$root/current
export YESH_RSYNC_TEMPORARY=$root/current.partial
export YESH_RSYNC_LOCK=$root/.lock

"$yesh" --plan "$example" >"$base/plan.out" 2>"$base/plan.err"
grep -F 'PLAN ["rsync" "-a" "--fsync" "--delay-updates" "--timeout=30"' "$base/plan.out" >/dev/null
test ! -e "$root/current"
test ! -e "$root/current.partial"

"$yesh" "$example" >"$base/positive.out" 2>"$base/positive.err"
test -d "$root/current"
test ! -e "$root/current.partial"
test ! -s "$base/positive.err"

export YESH_RSYNC_REMOTE_PATH=/tmp/yesh-rwe-missing-source
export YESH_RSYNC_SOURCE=max@192.168.111.113:/tmp/yesh-rwe-missing-source/
export YESH_RSYNC_CURRENT=$root/missing-current
export YESH_RSYNC_TEMPORARY=$root/missing-current.partial
if "$yesh" "$example" >"$base/negative.out" 2>"$base/negative.err"; then
    echo "ERROR: missing remote path was accepted" >&2
    exit 1
fi
test ! -e "$root/missing-current"
test ! -e "$root/missing-current.partial"

echo "RWE network preflight, remote rsync publication, plan, and missing-path negative passed"
