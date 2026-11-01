#!/bin/sh
# Prepare disposable user-level systemd units for yesh resource diagnostics.
# No sudo, package installation, system-unit changes, or direct cgroup writes.
# The units stay alive long enough for bounded snapshots and are always cleaned.

set -eu
umask 077

command -v systemd-run >/dev/null
command -v systemctl >/dev/null
command -v python3 >/dev/null

yesh_bin=${YESH_BIN:-/tmp/yesh}
if test ! -x "$yesh_bin"; then
    echo "SKIP: executable yesh not found: $yesh_bin" >&2
    exit 77
fi

if ! systemctl --user show-environment >/dev/null 2>&1; then
    echo "SKIP: user systemd manager is unavailable" >&2
    exit 77
fi

suffix=$$
stable="yesh-resource-$suffix-stable.service"
memory="yesh-resource-$suffix-memory.service"
growth="yesh-resource-$suffix-growth.service"
cpu="yesh-resource-$suffix-cpu.service"
restart="yesh-resource-$suffix-restart.service"
units="$stable $memory $growth $cpu $restart"
tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/yesh-resource-fixture.XXXXXX")

cleanup() {
    systemctl --user stop $units >/dev/null 2>&1 || true
    systemctl --user reset-failed $units >/dev/null 2>&1 || true
    rm -rf -- "$tmp_dir"
}
trap cleanup EXIT HUP INT TERM

systemd-run --user --unit="$stable" --collect \
    --property=MemoryMax=64M --property=CPUQuota=100% \
    /usr/bin/sleep 20

systemd-run --user --unit="$memory" --collect \
    --property=MemoryMax=64M \
    /usr/bin/python3 -c 'import time; blocks=[]; [blocks.append(bytearray(4*1024*1024)) or time.sleep(1) for _ in range(6)]; time.sleep(8)'

systemd-run --user --unit="$growth" --collect \
    /usr/bin/python3 -c 'exec("import time\nblocks=[]\nfor _ in range(12):\n b=bytearray(8*1024*1024)\n for i in range(0, len(b), 4096): b[i]=1\n blocks.append(b)\n time.sleep(1)\ntime.sleep(15)")'

systemd-run --user --unit="$cpu" --collect \
    --property=CPUQuota=20% \
    /usr/bin/python3 -c 'import time; end=time.monotonic()+15; exec("while time.monotonic() < end: pass")'

systemd-run --user --unit="$restart" --collect \
    --property=Restart=on-failure --property=RestartSec=1s \
    /bin/sh -c 'sleep 2; exit 1'

wait_active() {
    unit=$1
    attempts=0
    while ! systemctl --user is-active --quiet "$unit"; do
        attempts=$((attempts + 1))
        if test "$attempts" -ge 100; then
            echo "error: user unit did not become active: $unit" >&2
            systemctl --user status "$unit" --no-pager -n 20 >&2 || true
            exit 1
        fi
        sleep 0.1
    done
}

wait_active "$stable"
wait_active "$memory"
wait_active "$growth"
wait_active "$cpu"
wait_active "$restart"

snapshot() {
    unit=$1
    script="$tmp_dir/$unit.yesh"
    printf 'service resources user %s\n' "$unit" >"$script"
    "$yesh_bin" "$script"
}

echo "FIXTURE_UNITS=$units"
echo '--- snapshot 1 ---'
snapshot "$stable"
snapshot "$memory"
snapshot "$growth"
snapshot "$cpu"
snapshot "$restart"
sleep 2
echo '--- snapshot 2 ---'
snapshot "$stable"
snapshot "$memory"
snapshot "$growth"
snapshot "$cpu"
snapshot "$restart"
sleep 2
echo '--- snapshot 3 ---'
snapshot "$stable"
snapshot "$memory"
snapshot "$growth"
snapshot "$cpu"
snapshot "$restart"

default_home="$tmp_dir/home"
mkdir -p "$default_home"
export HOME="$default_home"
default_script="$tmp_dir/default-state.yesh"
printf 'measure service user %s for 2s every 1s\nsummarize service user %s since 1h\n' \
    "$stable" "$stable" >"$default_script"
"$yesh_bin" "$default_script"
default_state="$HOME/.local/.yesh/resources/$stable"
if test "$(stat -c '%a' "$default_state")" != 700; then
    echo "error: default state directory is not mode 700: $default_state" >&2
    exit 1
fi
echo "FIXTURE default user state verified: $default_state mode 700"

growth_script="$tmp_dir/growth-measurement.yesh"
growth_state="$tmp_dir/growth-state"
printf 'state_dir = "%s"\nmeasure service user %s for 20s every 2s save to $state_dir\nsummarize service user %s from $state_dir since 1h\n' \
    "$growth_state" "$growth" "$growth" >"$growth_script"
growth_output="$tmp_dir/growth-measurement.out"
"$yesh_bin" "$growth_script" >"$growth_output"
cat "$growth_output"
if ! grep -q 'trend rising' "$growth_output"; then
	echo 'error: intentional memory-growth fixture did not produce a rising memory trend' >&2
    exit 1
fi
echo 'FIXTURE snapshots completed; cleanup follows.'
