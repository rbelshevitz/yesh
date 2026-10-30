#!/bin/sh
# RWE B3: disposable config validation, atomic publication, and systemd check.
# Run on a disposable Linux host as root:
#   sudo sh /home/max/yesh-config-systemd-rwe.sh /home/max/yesh-rwe-new /home/max/config-systemd-rwe.yesh

set -eu
umask 077

yesh=${1:?usage: $0 /path/to/yesh /path/to/example}
example=${2:?usage: $0 /path/to/yesh /path/to/example}
test -x "$yesh"
test -r "$example"
test -x /usr/bin/systemctl
test -x /usr/bin/grep
test -x /usr/bin/install

base=$(mktemp -d /tmp/yesh-config-systemd-rwe.XXXXXX)
unit=/run/systemd/system/yesh-config-rwe.service
cleanup() {
    rc=$?
    systemctl stop yesh-config-rwe.service >/dev/null 2>&1 || true
    rm -f -- "$unit"
    systemctl daemon-reload >/dev/null 2>&1 || true
    case "$base" in
        /tmp/yesh-config-systemd-rwe.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

cat >"$unit" <<'UNIT'
[Unit]
Description=Disposable yesh config RWE service

[Service]
Type=simple
ExecStart=/usr/bin/sleep 300
UNIT
systemctl daemon-reload
systemctl start yesh-config-rwe.service

staged=$base/staged.conf
active=$base/active.conf
next=$base/active.conf.next
printf '%s\n' 'mode=stable' >"$active"
printf '%s\n' 'mode=production' >"$staged"

export YESH_CONFIG_STAGED=$staged
export YESH_CONFIG_ACTIVE=$active
export YESH_CONFIG_NEXT=$next

plan=$base/plan.out
if "$yesh" --plan "$example" >"$plan"; then
    grep -F 'PLAN' "$plan" >/dev/null
else
    echo 'ERROR: config-systemd --plan failed' >&2
    exit 1
fi
test "$(cat "$active")" = 'mode=stable'
test ! -e "$next"

"$yesh" "$example" >"$base/apply.out"
test "$(cat "$active")" = 'mode=production'
test ! -e "$next"
test "$(stat -c '%a' "$active")" = 640
systemctl is-active --quiet yesh-config-rwe.service
grep -F 'SERVICE_RESTART' "$base/apply.out" >/dev/null
grep -F 'SERVICE_STATUS' "$base/apply.out" >/dev/null

printf '%s\n' 'mode=invalid' >"$staged"
if "$yesh" "$example" >"$base/invalid.out" 2>&1; then
    echo 'ERROR: invalid configuration was accepted' >&2
    exit 1
fi
test "$(cat "$active")" = 'mode=production'
test ! -e "$next"

echo 'RWE config validation, plan, atomic publish, systemd postcondition, journal, and invalid-config negative passed'
