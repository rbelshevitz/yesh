#!/usr/bin/env bash
set -euo pipefail

# Run the guarded rsnapshot yesh example end to end.
# This wrapper does not edit rsnapshot.conf and does not create a mount.

usage() {
    echo "usage: $0 CONFIG LEVEL" >&2
    echo "   or: $0 CONFIG LEVEL SYNC_FIRST SNAPSHOT_ROOT MOUNT_SOURCE LOCKFILE PUBLISHED" >&2
    echo "automatic form reads safe values from rsnapshot.conf and findmnt" >&2
    exit 2
}

if [ "$#" -eq 2 ]; then
    config=$1
    level=$2

    [ -f "$config" ] || { echo "configuration is missing: $config" >&2; exit 2; }

    config_value() {
        local key=$1
        local value_count
        value_count=$(awk -F '\t' -v key="$key" '$1 == key { count++; value=$2 } END { if (count == 1) print value; else exit 10 }' "$config") || {
            echo "configuration must contain exactly one tab-separated $key value" >&2
            exit 2
        }
        printf '%s' "$value_count"
    }

    if grep -Eq '^[[:space:]]*include_conf([[:space:]]|$)' "$config"; then
        echo "automatic discovery refuses rsnapshot.conf with include_conf; use the explicit form" >&2
        exit 2
    fi

    root=$(config_value snapshot_root)
    lock=$(config_value lockfile)
    no_create_root=$(awk -F '\t' '$1 == "no_create_root" { count++; value=$2 } END { if (count > 1) exit 10; if (count == 1) print value; else print "" }' "$config") || {
        echo "configuration has ambiguous no_create_root values" >&2
        exit 2
    }
    [ "$no_create_root" = 1 ] || {
        echo "automatic discovery requires no_create_root 1 in rsnapshot.conf" >&2
        exit 2
    }
    [ "$root" != / ] || {
        echo "automatic discovery refuses snapshot_root=/" >&2
        exit 2
    }
    if [ "$level" = auto ]; then
        level=$(awk -F '\t' '$1 == "retain" { print $2; exit }' "$config")
        [ -n "$level" ] || { echo "configuration has no retain interval" >&2; exit 2; }
    fi
    sync_first=$(awk -F '\t' '$1 == "sync_first" { count++; value=$2 } END { if (count > 1) exit 10; if (count == 1) print value; else print "0" }' "$config") || {
        echo "configuration has ambiguous sync_first values" >&2
        exit 2
    }
    mount_source=$(findmnt -no UUID --target "$root" 2>/dev/null || true)
    if [ -n "$mount_source" ]; then
        mount_source="UUID=$mount_source"
    else
        mount_source=$(findmnt -no SOURCE --target "$root" 2>/dev/null || true)
    fi
    [ -n "$mount_source" ] || { echo "snapshot_root is not mounted: $root" >&2; exit 2; }
    published=${root%/}/$level.0
elif [ "$#" -eq 7 ]; then
    config=$1
    level=$2
    sync_first=$3
    root=$4
    mount_source=$5
    lock=$6
    published=$7
else
    usage
fi

awk -F '\t' -v level="$level" '$1 == "retain" && $2 == level { found=1 } END { exit(found ? 0 : 1) }' "$config" || {
    echo "interval is not declared by retain in rsnapshot.conf: $level" >&2
    echo "available intervals:" >&2
    awk -F '\t' '$1 == "retain" { print "  " $2 }' "$config" >&2
    exit 2
}

case "$sync_first" in
    0|1) ;;
    *) echo "SYNC_FIRST must be 0 or 1" >&2; exit 2 ;;
esac

[ -f "$config" ] || { echo "configuration is missing: $config" >&2; exit 2; }
[ -x "${YESH_BIN:-/home/max/yesh}" ] || { echo "yesh binary is missing or not executable" >&2; exit 2; }
[ -f "${YESH_SCRIPT:-/home/max/rsnapshot-apply.yesh}" ] || { echo "yesh script is missing" >&2; exit 2; }

yesh_bin=${YESH_BIN:-/home/max/yesh}
yesh_script=${YESH_SCRIPT:-/home/max/rsnapshot-apply.yesh}
state=$(mktemp -d /tmp/yesh-rsnapshot-run.XXXXXX)
report=$state/rsnapshot-test.log
confirm=$state/approved

umask 077
echo "Temporary state: $state"
echo "The confirmation file does not exist yet: $confirm"

sudo -v

run_yesh() {
    sudo env \
        YESH_RSNAPSHOT_CONFIG="$config" \
        YESH_RSNAPSHOT_LEVEL="$level" \
        YESH_RSNAPSHOT_SYNC_FIRST="$sync_first" \
        YESH_RSNAPSHOT_ROOT="$root" \
        YESH_RSNAPSHOT_SOURCE="$mount_source" \
        YESH_RSNAPSHOT_LOCK="$lock" \
        YESH_RSNAPSHOT_REPORT="$report" \
        YESH_RSNAPSHOT_PUBLISHED="$published" \
        YESH_RSNAPSHOT_CONFIRM="$confirm" \
        "$yesh_bin" "$@" "$yesh_script"
}

echo
echo '=== yesh plan (no commands are run) ==='
run_yesh --plan

echo
echo '=== preflight and rsnapshot test mode ==='
set +e
run_yesh
status=$?
set -e
if [ "$status" -ne 4 ]; then
    echo "expected the preflight to stop at confirmation with code 4; got $status" >&2
    exit "$status"
fi

echo
echo "=== test report: $report ==="
if command -v less >/dev/null 2>&1; then
    less "$report"
else
    sed -n '1,240p' "$report"
fi

read -r -p 'Run the changing rsnapshot operation now? [y/N] ' answer
case "$answer" in
    y|Y|yes|YES) ;;
    *) echo "Not confirmed. State preserved in $state"; exit 4 ;;
esac

touch -- "$confirm"
echo
echo '=== confirmed rsnapshot operation ==='
run_yesh

echo
echo "Completed. Report and confirmation state remain in $state"
