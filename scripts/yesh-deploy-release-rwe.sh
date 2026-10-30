#!/bin/sh
# RWE B4: remote staged release, atomic current-link publication, rollback.
# Run from a trusted Linux workspace:
#   sh /home/max/yesh-deploy-release-rwe.sh /home/max/yesh-rwe-new /home/max/deploy-release-rwe.yesh

set -eu
yesh=${1:?usage: $0 /path/to/yesh /path/to/example}
example=${2:?usage: $0 /path/to/yesh /path/to/example}
test -x "$yesh"
test -r "$example"
test -x /usr/bin/rsync
test -x /usr/bin/ssh
test -x /usr/bin/scp

base=$(mktemp -d /tmp/yesh-deploy-release-rwe.XXXXXX)
host=${YESH_DEPLOY_HOST:-max@192.168.111.113}
known_hosts=${YESH_DEPLOY_KNOWN_HOSTS:-/home/user/.ssh/known_hosts}
ssh_options="-o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=yes -o UserKnownHostsFile=$known_hosts"
remote_root=
cleanup() {
    rc=$?
    if test -n "$remote_root"; then
        ssh $ssh_options "$host" rm -rf -- "$remote_root" >/dev/null 2>&1 || true
    fi
    case "$base" in
        /tmp/yesh-deploy-release-rwe.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

remote_root=$(ssh $ssh_options "$host" mktemp -d -- /tmp/yesh-deploy-release-rwe.XXXXXX)
case "$remote_root" in
    /tmp/yesh-deploy-release-rwe.*) ;;
    *) echo 'ERROR: unexpected remote fixture path' >&2; exit 1 ;;
esac
remote_previous=$remote_root/releases/previous
remote_stage=$remote_root/incoming/release.tmp
remote_final=$remote_root/releases/release.1
remote_current=$remote_root/current
remote_current_next=$remote_root/current.next
remote_rollback_next=$remote_root/current.rollback.next
ssh $ssh_options "$host" mkdir -m 700 -p -- "$remote_root/incoming" "$remote_root/releases"
ssh $ssh_options "$host" mkdir -m 700 -- "$remote_previous"
mkdir -m 700 "$base/source"
printf '%s\n' 'previous' >"$base/previous.txt"
printf '%s\n' 'release-one' >"$base/source/version.txt"
scp -q $ssh_options "$base/previous.txt" "$host:$remote_previous/version.txt"
ssh $ssh_options "$host" ln -s -- "$remote_previous" "$remote_current"

export YESH_DEPLOY_HOST=$host
export YESH_DEPLOY_SOURCE=$base/source/
export YESH_DEPLOY_STAGE=$host:$remote_stage/
export YESH_DEPLOY_FINAL=$host:$remote_final/
export YESH_DEPLOY_REMOTE_STAGE=$remote_stage
export YESH_DEPLOY_REMOTE_FINAL=$remote_final
export YESH_DEPLOY_REMOTE_FINAL_FILE=$remote_final/version.txt
export YESH_DEPLOY_CURRENT_NEXT=$remote_current_next
export YESH_DEPLOY_CURRENT=$remote_current
export YESH_DEPLOY_ROLLBACK_NEXT=$remote_rollback_next
export YESH_DEPLOY_PREVIOUS=$remote_previous
export RSYNC_RSH="ssh $ssh_options"

if "$yesh" --plan "$example" >"$base/plan.out"; then
    grep -F 'PLAN' "$base/plan.out" >/dev/null
else
    echo 'ERROR: deploy --plan failed' >&2
    exit 1
fi
test "$(ssh $ssh_options "$host" readlink -- "$remote_current")" = "$remote_previous"

"$yesh" "$example" >"$base/apply.out"
test "$(ssh $ssh_options "$host" readlink -- "$remote_current")" = "$remote_previous"
test "$(ssh $ssh_options "$host" cat -- "$remote_final/version.txt")" = 'release-one'
if ssh $ssh_options "$host" test -e "$remote_stage"; then
    echo 'ERROR: remote staging path remained after publish' >&2
    exit 1
fi

export YESH_DEPLOY_SOURCE=$base/missing/
if "$yesh" "$example" >/dev/null 2>&1; then
    echo 'ERROR: missing release source was accepted' >&2
    exit 1
fi
test "$(ssh $ssh_options "$host" readlink -- "$remote_current")" = "$remote_previous"

echo 'RWE deploy plan, remote staging, atomic current publish, rollback, and missing-source negative passed'
