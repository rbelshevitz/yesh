#!/bin/sh
# RWE B8: selected snapshot restore into an empty disposable target.
# Run on the Linux test host:
#   sh /home/max/yesh-selected-restore-rwe.sh /home/max/yesh-rwe-new /home/max/rsync-selected-restore.yesh

set -eu
umask 077

yesh=${1:?usage: $0 /path/to/yesh /path/to/example}
example=${2:?usage: $0 /path/to/yesh /path/to/example}
test -x "$yesh"
test -r "$example"
test -x /usr/bin/rsync
test -x /usr/bin/findmnt
test -x /usr/bin/sha256sum

base=$(mktemp -d /tmp/yesh-selected-restore-rwe.XXXXXX)
cleanup() {
    rc=$?
    case "$base" in
        /tmp/yesh-selected-restore-rwe.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

root=$base/snapshots
source=$base/source
target=$base/restore
nonempty=$base/nonempty
mkdir -m 700 "$root" "$source" "$target" "$nonempty"
mkdir -m 700 "$root/snapshot.1" "$root/snapshot.2"
printf '%s\n' 'version one' >"$root/snapshot.1/version.txt"
printf '%s\n' 'version two' >"$root/snapshot.2/version.txt"
printf '%s\n' 'stable' >"$root/snapshot.2/stable.txt"
printf '%s\n' 'sentinel' >"$nonempty/sentinel.txt"
ln -s snapshot.2 "$root/snapshot-link"

export YESH_RESTORE_SNAPSHOT=$root/snapshot.2
export YESH_RESTORE_SOURCE=$root/snapshot.2/
export YESH_RESTORE_TARGET=$target

plan=$base/plan.out
if "$yesh" --plan "$example" >"$plan"; then
    grep -F 'PLAN' "$plan" >/dev/null
else
    echo 'ERROR: restore --plan failed' >&2
    exit 1
fi
test ! -e "$target/version.txt"

"$yesh" "$example"
test "$(cat "$target/version.txt")" = 'version two'
test "$(cat "$target/stable.txt")" = 'stable'
test "$(sha256sum "$target/version.txt" | cut -d ' ' -f 1)" = "$(sha256sum "$root/snapshot.2/version.txt" | cut -d ' ' -f 1)"

export YESH_RESTORE_SNAPSHOT=$root/snapshot-link
export YESH_RESTORE_SOURCE=$root/snapshot-link/
if "$yesh" "$example" >/dev/null 2>&1; then
    echo 'ERROR: symlink restore source was accepted' >&2
    exit 1
fi
test "$(cat "$target/version.txt")" = 'version two'

export YESH_RESTORE_SNAPSHOT=$root/snapshot.1
export YESH_RESTORE_SOURCE=$root/snapshot.1/
export YESH_RESTORE_TARGET=$nonempty
if "$yesh" "$example" >/dev/null 2>&1; then
    echo 'ERROR: non-empty restore target was accepted' >&2
    exit 1
fi
test "$(cat "$nonempty/sentinel.txt")" = 'sentinel'
test ! -e "$nonempty/version.txt"

export YESH_RESTORE_SNAPSHOT=$root/missing
export YESH_RESTORE_SOURCE=$root/missing/
export YESH_RESTORE_TARGET=$target
if "$yesh" "$example" >/dev/null 2>&1; then
    echo 'ERROR: missing restore source was accepted' >&2
    exit 1
fi
test "$(cat "$target/version.txt")" = 'version two'

echo 'RWE selected restore, plan, symlink, non-empty-target, and missing-source negatives passed'
