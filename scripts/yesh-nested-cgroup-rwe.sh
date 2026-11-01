#!/bin/sh

# Run a disposable nested cgroup v2 resource fixture.
#
# The script needs root because it starts a transient system service with
# delegated controllers. It creates no persistent unit file, does not change
# an existing service, and removes its temporary cgroup and I/O file on exit.
# The output is intentionally raw: compare the parent and child values instead
# of treating them as one number.

set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo "run as root, for example: sudo $0" >&2
    exit 2
fi

if ! command -v systemd-run >/dev/null 2>&1; then
    echo "systemd-run is required" >&2
    exit 2
fi

if [ "$(stat -fc %T /sys/fs/cgroup 2>/dev/null || true)" != "cgroup2fs" ]; then
    echo "unified cgroup v2 is not mounted at /sys/fs/cgroup" >&2
    exit 4
fi

unit="yesh-nested-cgroup-$$.service"

systemd-run \
    --unit="$unit" \
    --collect \
    --pipe \
    --wait \
    --property=Delegate=yes \
    --property=IOAccounting=yes \
    --property=RuntimeMaxSec=90s \
    /bin/sh -s <<'FIXTURE'
set -eu

cg=$(awk -F: '$1 == "0" {print $3}' /proc/self/cgroup)
base=/sys/fs/cgroup$cg
child=$base/yesh-child
iofile=$(mktemp /tmp/yesh-nested-cgroup.XXXXXX)

cleanup() {
    printf '%s' "$$" > "$base/cgroup.procs" 2>/dev/null || true
    rm -f "$iofile"
    rmdir "$child" 2>/dev/null || true
}
trap cleanup EXIT HUP INT TERM

mkdir "$child"

# Leave the parent empty before enabling domain controllers for its child.
printf '%s' "$$" > "$child/cgroup.procs"
printf '%s' '+cpu +memory +io' > "$base/cgroup.subtree_control"

echo "cgroup=$cg"
printf 'available_controllers='
tr '\n' ' ' < "$base/cgroup.controllers"
printf '\n'
printf 'enabled_for_children='
tr '\n' ' ' < "$base/cgroup.subtree_control"
printf '\n'

for file in memory.current cpu.stat io.stat; do
    printf 'parent_before_%s=\n' "$file"
    if [ -r "$base/$file" ]; then
        cat "$base/$file"
    else
        echo unavailable
    fi
    printf 'child_before_%s=\n' "$file"
    if [ -r "$child/$file" ]; then
        cat "$child/$file"
    else
        echo unavailable
    fi
done

if ! dd if=/dev/zero of="$iofile" bs=1M count=32 oflag=direct conv=fdatasync status=none; then
    dd if=/dev/zero of="$iofile" bs=1M count=32 conv=fdatasync status=none
fi

printf 'parent_after_memory.current=\n'; cat "$base/memory.current"
printf 'child_after_memory.current=\n'; cat "$child/memory.current"
printf 'parent_after_cpu.stat=\n'; cat "$base/cpu.stat"
printf 'child_after_cpu.stat=\n'; cat "$child/cpu.stat"
printf 'parent_after_io.stat=\n'
if [ -r "$base/io.stat" ]; then cat "$base/io.stat"; else echo unavailable; fi
printf 'child_after_io.stat=\n'
if [ -r "$child/io.stat" ]; then cat "$child/io.stat"; else echo unavailable; fi
FIXTURE
