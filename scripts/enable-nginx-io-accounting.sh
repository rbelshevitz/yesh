#!/bin/sh

# Enable temporary cgroup v2 I/O accounting for nginx.service.
#
# This changes the running systemd manager state only. The setting is not
# persisted and disappears after the manager or host is restarted. The script
# does not restart nginx, change I/O limits, or write a unit drop-in.

set -eu

unit=${1:-nginx.service}

if [ "$(id -u)" -ne 0 ]; then
    echo "run as root, for example: sudo $0 $unit" >&2
    exit 2
fi

if ! command -v systemctl >/dev/null 2>&1; then
    echo "systemctl is required" >&2
    exit 2
fi

if [ "$(stat -fc %T /sys/fs/cgroup 2>/dev/null || true)" != "cgroup2fs" ]; then
    echo "unified cgroup v2 is not mounted at /sys/fs/cgroup" >&2
    exit 4
fi

if ! systemctl is-active --quiet "$unit"; then
    echo "unit is not active: $unit" >&2
    exit 4
fi

control_group=$(systemctl show --value --property=ControlGroup "$unit")
if [ -z "$control_group" ] || [ "${control_group#/}" = "$control_group" ]; then
    echo "unit has no valid systemd ControlGroup: $unit" >&2
    exit 4
fi

cgroup_path=/sys/fs/cgroup$control_group
if [ ! -d "$cgroup_path" ]; then
    echo "cgroup path does not exist: $cgroup_path" >&2
    exit 4
fi

echo "Enabling temporary IOAccounting for $unit"
systemctl set-property --runtime "$unit" IOAccounting=yes

if [ ! -r "$cgroup_path/io.stat" ]; then
    echo "IOAccounting was accepted, but io.stat is still unavailable: $cgroup_path" >&2
    echo "Check the systemd journal and cgroup controllers before treating this as a yesh failure." >&2
    exit 1
fi

echo "io.stat is available: $cgroup_path/io.stat"
echo ""
echo "Controllers:"
cat "$cgroup_path/cgroup.controllers"
echo ""
echo "I/O counters:"
cat "$cgroup_path/io.stat"
echo ""
echo "The setting is runtime-only. Run yesh service resources again now."
