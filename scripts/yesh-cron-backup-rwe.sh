#!/bin/sh
set -eu
umask 077

if test "$#" -ne 2; then
    echo "usage: $0 /path/to/yesh /path/to/cron-backup-rwe.yesh" >&2
    exit 2
fi
yesh=$1
example=$2
test -x "$yesh"
test -r "$example"
test -x /usr/bin/flock
test -x /usr/bin/timeout

base=$(mktemp -d /tmp/yesh-cron-backup-rwe.XXXXXX)
trap 'rm -rf -- "$base"' EXIT INT TERM HUP
root=$base/root
source=$base/source
mkdir "$root" "$source"
printf '%s\n' payload >"$source/file"

run() {
    env -i HOME="$base/home" LOGNAME="yesh" PATH=/usr/bin:/bin LC_ALL=C \
        YESH_CRON_ROOT="$root" YESH_CRON_SOURCE="$source/" \
        YESH_CRON_TEMPORARY="$root/current.partial" \
        YESH_CRON_CURRENT="$root/current" \
        YESH_CRON_RESULT="$root/result" \
        YESH_CRON_RESULT_PARTIAL="$root/result.partial" \
        YESH_CRON_INNER_LOCK="$root/inner.lock" \
        YESH_CRON_DELAY="$1" \
        /usr/bin/flock -n -E 75 "$root/cron.lock" \
        /usr/bin/timeout --signal=TERM --kill-after=2s "$2" \
        "$yesh" "$example"
}

run 0s 10s >"$base/positive.out" 2>"$base/positive.err"
test -f "$root/current/file"
test "$(/bin/cat "$root/current/file")" = payload
test "$(/bin/cat "$root/result")" = run=started
test ! -e "$root/current.partial"
test ! -e "$root/result.partial"
test ! -s "$base/positive.err"

exec 9>"$root/cron.lock"
/usr/bin/flock -n 9
set +e
run 0s 10s >"$base/locked.out" 2>"$base/locked.err"
locked_status=$?
set -e
test "$locked_status" = 75
exec 9>&-

rm -rf -- "$root/current" "$root/result" "$source"
set +e
run 0s 10s >"$base/missing.out" 2>"$base/missing.err"
missing_status=$?
set -e
test "$missing_status" != 0
test ! -e "$root/current"
test ! -e "$root/result"

mkdir "$source"
printf '%s\n' payload >"$source/file"
set +e
run 5s 1s >"$base/timeout.out" 2>"$base/timeout.err"
timeout_status=$?
set -e
test "$timeout_status" = 124
test ! -e "$root/current"
test ! -e "$root/result"
test -e "$root/result.partial"

echo "RWE cron clean environment, outer lock, timeout, result, and no-publish negatives passed"
