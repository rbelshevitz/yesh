#!/usr/bin/env bash
# Comparative baseline for examples/rsync-incremental.yesh.
#
# Run only against a disposable fixture. The script is intentionally explicit:
# it shows the Bash bookkeeping that yesh keeps visible in the scenario itself.
# Required variables:
#   YESH_RSYNC_SOURCE     local or remote rsync source; keep a trailing slash
#   YESH_RSYNC_ROOT       local snapshot root
#   YESH_RSYNC_PREVIOUS   previous snapshot, or a missing path for the first run
#   YESH_RSYNC_CURRENT    new published snapshot; must not exist
#   YESH_RSYNC_TEMPORARY  private staging directory; must not exist
#   YESH_RSYNC_LOCK       lock path under the snapshot root

set -Eeuo pipefail
IFS=$'\n\t'

die() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

for name in \
    YESH_RSYNC_SOURCE \
    YESH_RSYNC_ROOT \
    YESH_RSYNC_PREVIOUS \
    YESH_RSYNC_CURRENT \
    YESH_RSYNC_TEMPORARY \
    YESH_RSYNC_LOCK
do
    [[ -n ${!name-} ]] || die "missing environment variable: $name"
done

source=$YESH_RSYNC_SOURCE
root=$YESH_RSYNC_ROOT
previous=$YESH_RSYNC_PREVIOUS
current=$YESH_RSYNC_CURRENT
temporary=$YESH_RSYNC_TEMPORARY
lock=$YESH_RSYNC_LOCK
temporary_created=0

cleanup() {
    rc=$?
    trap - EXIT

    if (( temporary_created )) && [[ -d $temporary ]]; then
        if ! rm -rf -- "$temporary"; then
            printf 'error: could not remove temporary path: %s\n' "$temporary" >&2
            rc=1
        fi
    fi
    exit "$rc"
}
trap cleanup EXIT

[[ -d $root ]] || die "snapshot root is missing: $root"
root=$(realpath -e -- "$root") || die "cannot resolve snapshot root"
current=$(realpath -m -- "$current") || die "cannot resolve published path"
temporary=$(realpath -m -- "$temporary") || die "cannot resolve temporary path"
lock=$(realpath -m -- "$lock") || die "cannot resolve lock path"
if [[ -e $previous ]]; then
    previous=$(realpath -e -- "$previous") || die "cannot resolve previous snapshot"
fi

[[ ! -e $current ]] || die "published snapshot already exists: $current"
[[ ! -e $temporary ]] || die "temporary path already exists: $temporary"
[[ $current != "$temporary" ]] || die "published and temporary paths are identical"
[[ $temporary == "$root"/* ]] || die "temporary path is outside snapshot root"
[[ $current == "$root"/* ]] || die "published path is outside snapshot root"
[[ $lock == "$root"/* ]] || die "lock path is outside snapshot root"

root_device=$(stat -c '%d' -- "$root") || die "cannot inspect snapshot root"
temporary_parent=${temporary%/*}
[[ -n $temporary_parent && -d $temporary_parent ]] ||
    die "temporary parent is missing: $temporary_parent"
temporary_device=$(stat -c '%d' -- "$temporary_parent") ||
    die "cannot inspect temporary parent"
[[ $root_device == "$temporary_device" ]] ||
    die "temporary path is not on the snapshot filesystem"

if [[ -e $previous && ! -d $previous ]]; then
    die "previous snapshot is not a directory: $previous"
fi

exec 9>"$lock" || die "cannot open lock file: $lock"
flock -n 9 || die "another backup holds the lock: $lock"

mkdir -- "$temporary"
temporary_created=1

rsync_args=(-a --fsync --delay-updates)
if [[ -e $previous ]]; then
    rsync_args+=(--link-dest="$previous")
fi

rsync "${rsync_args[@]}" -- "$source" "$temporary/"

# The destination name did not exist above; same-filesystem rename publishes
# the completed tree without copying it over an existing snapshot.
mv -- "$temporary" "$current"
temporary_created=0

printf 'published snapshot: %s\n' "$current"
