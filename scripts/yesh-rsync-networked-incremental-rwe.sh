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
test -x /usr/bin/ssh
test -x /usr/bin/rsync
test -x /usr/bin/ssh-keyscan
test -x /usr/bin/ssh-keygen
test -r /etc/ssh/ssh_host_ed25519_key.pub

base=$(mktemp -d /tmp/yesh-rsync-networked-incremental-rwe.XXXXXX)
remote_dir=/tmp/yesh-rsync-networked-incremental-rwe.$$
home=$base/home
known_hosts=$home/.ssh/known_hosts
original_home=${HOME:-/home/max}
cleanup() {
    rc=$?
    HOME="$home" ssh $ssh_options "$YESH_RSYNC_HOST" "rm -rf -- '$remote_dir'" >/dev/null 2>&1 || rc=1
    rm -rf -- "$base"
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP
root=$base/snapshots
mkdir -p "$root" "$home/.ssh"
test -r "$original_home/.ssh/id_ed25519"
ln -s "$original_home/.ssh/id_ed25519" "$home/.ssh/id_ed25519"

/usr/bin/ssh-keyscan -T 5 -t ed25519 192.168.111.113 2>/dev/null >"$known_hosts"
/usr/bin/awk '{print $2 " " $3}' "$known_hosts" >"$base/scanned.pub"
scanned_fingerprint=$(/usr/bin/ssh-keygen -lf "$base/scanned.pub" | /usr/bin/awk '{print $2}')
host_fingerprint=$(/usr/bin/ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub | /usr/bin/awk '{print $2}')
[ -n "$scanned_fingerprint" ]
[ "$scanned_fingerprint" = "$host_fingerprint" ] || {
    echo "ERROR: scanned SSH host key does not match the local host key" >&2
    exit 1
}

ssh_options="-o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=yes -o UserKnownHostsFile=$known_hosts -o GlobalKnownHostsFile=/dev/null"
export HOME=$home

export YESH_RSYNC_DNS_NAME=192.168.111.113
export YESH_RSYNC_ROUTE_TARGET=192.168.111.113
export YESH_RSYNC_HOST=max@192.168.111.113
export YESH_RSYNC_REMOTE_PATH=$remote_dir
export YESH_RSYNC_SOURCE=max@192.168.111.113:$remote_dir/
export YESH_RSYNC_ROOT=$root
export YESH_RSYNC_CURRENT=$root/current
export YESH_RSYNC_TEMPORARY=$root/current.partial
export YESH_RSYNC_LOCK=$root/.lock

HOME="$home" ssh $ssh_options "$YESH_RSYNC_HOST" "mkdir -m 700 -- '$remote_dir'; printf '%s\\n' remote-data > '$remote_dir/data.txt'"

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

HOME="$home" ssh $ssh_options "$YESH_RSYNC_HOST" "printf '%s\\n' readable > '$remote_dir/readable.txt'; printf '%s\\n' secret > '$remote_dir/secret.txt'; chmod 000 '$remote_dir/secret.txt'"
export YESH_RSYNC_REMOTE_PATH=$remote_dir
export YESH_RSYNC_SOURCE=max@192.168.111.113:$remote_dir/
export YESH_RSYNC_CURRENT=$root/partial-current
export YESH_RSYNC_TEMPORARY=$root/partial-current.partial
if "$yesh" "$example" >"$base/partial.out" 2>"$base/partial.err"; then
    echo "ERROR: partial remote transfer was accepted" >&2
    exit 1
fi
test ! -e "$root/partial-current"
test -e "$root/partial-current.partial"

HOME="$home" ssh $ssh_options "$YESH_RSYNC_HOST" "chmod 600 '$remote_dir/secret.txt'"
export YESH_RSYNC_CURRENT=$root/retry-current
export YESH_RSYNC_TEMPORARY=$root/retry-current.partial
"$yesh" "$example" >"$base/retry.out" 2>"$base/retry.err"
test -f "$root/retry-current/secret.txt"
test ! -e "$root/retry-current.partial"

echo "RWE network preflight, remote rsync publication, missing-path and partial-transfer negatives, and retry passed"
