#!/usr/bin/env bash
set -euo pipefail

# Run the disposable rsnapshot loopback/ext4 acceptance stand on another host.
# The caller supplies SSH authentication; no key or password is stored here.
# The remote sudo may prompt for the operator's password.

host=${1:-${YESH_SSH_TEST_HOST:-}}
yesh_bin=${YESH_BIN:-./yesh}
repo=${YESH_REPO:-.}
ssh_options=(-o BatchMode=no -o ConnectTimeout=5 -o StrictHostKeyChecking=yes)

die() {
    echo "run-yesh-rsnapshot-loopback-remote: $*" >&2
    exit 1
}

[ -n "$host" ] || die "usage: $0 user@host [YESH_SSH_TEST_HOST=user@host]"
[ -x "$yesh_bin" ] || die "yesh binary is not executable: $yesh_bin"
[ -f "$repo/examples/rsnapshot-apply.yesh" ] || die "missing rsnapshot example under $repo"
[ -x "$repo/scripts/yesh-rsnapshot-loopback-rwe.sh" ] || die "missing loopback runner under $repo"

remote_stage=$(ssh "${ssh_options[@]}" "$host" 'mktemp -d /tmp/yesh-rsnapshot-remote.XXXXXX')
remote_stage=${remote_stage//$'\r'/}
remote_stage=${remote_stage//$'\n'/}
[ "${remote_stage#/tmp/yesh-rsnapshot-remote.}" != "$remote_stage" ] || die "unexpected remote staging path: $remote_stage"

cleanup() {
    status=$?
    ssh "${ssh_options[@]}" "$host" rm -rf -- "$remote_stage" >/dev/null 2>&1 || true
    exit "$status"
}
trap cleanup EXIT INT TERM

scp "${ssh_options[@]}" "$yesh_bin" "$host:$remote_stage/yesh"
scp "${ssh_options[@]}" "$repo/examples/rsnapshot-apply.yesh" "$host:$remote_stage/rsnapshot-apply.yesh"
scp "${ssh_options[@]}" "$repo/scripts/yesh-rsnapshot-loopback-rwe.sh" "$host:$remote_stage/loopback-rwe.sh"
ssh "${ssh_options[@]}" "$host" chmod 0700 -- "$remote_stage/loopback-rwe.sh" "$remote_stage/yesh"

echo "Checking remote prerequisites..."
echo "Running loopback/ext4 acceptance stand on $host..."
ssh "${ssh_options[@]}" -tt "$host" "sudo -v && \
    sudo sh -c 'command -v losetup' && \
    sudo sh -c 'command -v rsnapshot' && \
    sudo sh -c 'command -v rsync' && \
    sudo env YESH_BIN=$remote_stage/yesh \
        YESH_SCRIPT=$remote_stage/rsnapshot-apply.yesh \
        $remote_stage/loopback-rwe.sh"
