#!/bin/bash
# Run on a disposable Linux host as root:
#   sudo bash scripts/yesh-fixture-test.sh
#
# The fixture lives in /run, never in /etc. It is removed automatically.

set -u

if test "$(id -u)" != 0; then
  echo "error: run this script through sudo on the Linux host" >&2
  exit 2
fi

yesh_bin=${YESH_BIN:-/tmp/yesh}
unit_dir=/run/systemd/system
normal=yesh-codex-fixture.service
oneshot=yesh-codex-fixture-oneshot.service
slow=yesh-codex-fixture-slow.service
fail=yesh-codex-fixture-fail.service
units="$normal $oneshot $slow $fail"
files="$unit_dir/$normal $unit_dir/$oneshot $unit_dir/$slow $unit_dir/$fail"

if test ! -x "$yesh_bin"; then
  echo "error: executable yesh not found: $yesh_bin" >&2
  exit 2
fi

tmp_dir=$(mktemp -d /tmp/yesh-fixture.XXXXXX)

for file in $files; do
  if test -e "$file"; then
    echo "error: refusing to overwrite existing file: $file" >&2
    exit 2
  fi
done

cleanup() {
  systemctl stop $units >/dev/null 2>&1 || true
  rm -f $files
  systemctl daemon-reload >/dev/null
  rm -rf -- "$tmp_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

cat >"$unit_dir/$normal" <<'UNIT'
[Unit]
Description=Disposable yesh systemd fixture

[Service]
Type=simple
ExecStart=/usr/bin/sleep 300
UNIT

cat >"$unit_dir/$oneshot" <<'UNIT'
[Unit]
Description=Disposable yesh oneshot fixture

[Service]
Type=oneshot
ExecStart=/usr/bin/true
UNIT

cat >"$unit_dir/$slow" <<'UNIT'
[Unit]
Description=Disposable yesh slow fixture

[Service]
Type=oneshot
ExecStart=/usr/bin/sleep 30
RemainAfterExit=yes
UNIT

cat >"$unit_dir/$fail" <<'UNIT'
[Unit]
Description=Disposable yesh failing fixture

[Service]
Type=oneshot
ExecStart=/bin/sh -c 'exit 23'
UNIT

systemctl daemon-reload

echo '--- start: expected success ---'
echo 'service start yesh-codex-fixture.service until active within 10s' >"$tmp_dir/start.yesh"
"$yesh_bin" "$tmp_dir/start.yesh"

echo '--- status: expected success ---'
printf '%s\n' 'service status yesh-codex-fixture.service' >"$tmp_dir/status.yesh"
"$yesh_bin" "$tmp_dir/status.yesh"

echo '--- wait: expected success ---'
printf '%s\n' 'service wait yesh-codex-fixture.service until active within 10s' >"$tmp_dir/wait.yesh"
"$yesh_bin" "$tmp_dir/wait.yesh"

echo '--- restart: expected success ---'
printf '%s\n' 'service restart yesh-codex-fixture.service until active within 10s' >"$tmp_dir/restart.yesh"
"$yesh_bin" "$tmp_dir/restart.yesh"

echo '--- oneshot without RemainAfterExit: expected failure ---'
printf '%s\n' 'service restart yesh-codex-fixture-oneshot.service until active within 5s' >"$tmp_dir/oneshot.yesh"
set +e
"$yesh_bin" "$tmp_dir/oneshot.yesh"
oneshot_status=$?
set -u
printf 'ONESHOT_EXIT=%s\n' "$oneshot_status"

echo '--- activating timeout: expected failure ---'
printf '%s\n' 'service wait yesh-codex-fixture-slow.service until active within 2s' >"$tmp_dir/slow.yesh"
systemctl start --no-block "$slow"
set +e
"$yesh_bin" "$tmp_dir/slow.yesh"
slow_status=$?
set -u
printf 'SLOW_EXIT=%s\n' "$slow_status"

echo '--- failed restart with diagnostic snapshot: expected failure ---'
printf '%s\n' 'service restart yesh-codex-fixture-fail.service until active within 5s' >"$tmp_dir/fail.yesh"
set +e
fail_output=$("$yesh_bin" "$tmp_dir/fail.yesh" 2>&1)
fail_status=$?
set -u
printf '%s\n' "$fail_output"
printf 'FAIL_EXIT=%s\n' "$fail_status"

echo '--- observed states before automatic cleanup ---'
systemctl show --no-pager --property=Id,LoadState,ActiveState,SubState $units

if test "$oneshot_status" -eq 0; then
  echo 'error: oneshot unexpectedly passed' >&2
  exit 1
fi
if test "$slow_status" -eq 0; then
  echo 'error: slow wait unexpectedly passed' >&2
  exit 1
fi
if test "$fail_status" -eq 0 || ! printf '%s' "$fail_output" | grep -q 'result=exit-code' || ! printf '%s' "$fail_output" | grep -q 'exec_main_status=23'; then
  echo 'error: failed restart did not preserve the diagnostic snapshot' >&2
  exit 1
fi

echo 'fixture checks passed; cleanup will run now'
