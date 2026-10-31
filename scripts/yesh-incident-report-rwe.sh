#!/bin/sh
set -eu
umask 077

yesh=${1:-}
example=${2:-}

if test -z "$yesh" || test -z "$example"; then
    echo "usage: $0 /path/to/yesh /path/to/incident-report-rwe.yesh" >&2
    exit 2
fi
test -x "$yesh"
test -r "$example"
test -x /usr/bin/systemctl
test -x /usr/bin/journalctl
test -x /usr/bin/findmnt
test -x /bin/df || test -x /usr/bin/df
test -x /bin/tar || test -x /usr/bin/tar

base=$(mktemp -d /tmp/yesh-incident-report-rwe.XXXXXX)
unit=/run/systemd/system/yesh-incident-rwe.service
cleanup() {
    rc=$?
    systemctl stop yesh-incident-rwe.service >/dev/null 2>&1 || true
    systemctl reset-failed yesh-incident-rwe.service >/dev/null 2>&1 || true
    rm -f -- "$unit"
    systemctl daemon-reload >/dev/null 2>&1 || true
    rm -rf -- "$base"
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

cat >"$unit" <<'UNIT'
[Unit]
Description=Disposable failing yesh incident fixture

[Service]
Type=oneshot
ExecStart=/bin/sh -c "echo disposable failure; exit 1"
UNIT
systemctl daemon-reload
systemctl reset-failed yesh-incident-rwe.service >/dev/null 2>&1 || true
systemctl start yesh-incident-rwe.service >/dev/null 2>&1 || true
test "$(systemctl is-failed yesh-incident-rwe.service)" = failed

plan=$base/plan.out
if ! "$yesh" --plan "$example" >"$plan" 2>"$base/plan.err"; then
    echo "ERROR: incident report --plan failed" >&2
    exit 1
fi
grep -F 'PLAN ["systemctl" "show"' "$plan" >/dev/null
grep -F 'PLAN ["journalctl" "--no-pager" "--output=json"' "$plan" >/dev/null
test ! -e "$base/report.tar"

if ! "$yesh" "$example" >"$base/report.out" 2>"$base/report.err"; then
    echo "ERROR: incident report execution failed; report is incomplete" >&2
    exit 1
fi
grep -F 'SERVICE_STATUS id=yesh-incident-rwe.service' "$base/report.out" >/dev/null
grep -F 'PROCESS_COUNT' "$base/report.out" >/dev/null
grep -F 'Filesystem' "$base/report.out" >/dev/null
test ! -s "$base/report.err"

tar -cf "$base/report.tar" -C "$base" plan.out plan.err report.out report.err
tar -tf "$base/report.tar" | grep -Fx 'report.out' >/dev/null
tar -tf "$base/report.tar" | grep -Fx 'report.err' >/dev/null
test -s "$base/report.tar"

# A missing unit is a negative path: it must not become a successful bundle.
bad=$base/bad.yesh
sed 's/yesh-incident-rwe\.service/yesh-rwe-definitely-missing.service/g' "$example" >"$bad"
"$yesh" "$bad" >"$base/bad.out" 2>"$base/bad.err" || true
grep -F 'load_state=not-found' "$base/bad.out" >/dev/null
test ! -e "$base/bad.tar"

echo "RWE incident report plan, bounded journal, process/storage snapshot, separate stderr, archive, and missing-unit negative passed"
