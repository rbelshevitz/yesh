#!/bin/sh
# B9 endpoint RWE: bounded DNS lookup and TCP connect without mutation.
# Usage: sh scripts/yesh-network-endpoint-rwe.sh /path/to/yesh /path/to/example

set -eu
umask 077

yesh=${1:?usage: $0 /path/to/yesh /path/to/network-endpoint-preflight.yesh}
example=${2:?usage: $0 /path/to/yesh /path/to/network-endpoint-preflight.yesh}
test -x "$yesh"
test -r "$example"
command -v getent >/dev/null
command -v nc >/dev/null
command -v tar >/dev/null

base=$(mktemp -d /tmp/yesh-network-endpoint-rwe.XXXXXX)
archive=${YESH_NETWORK_ENDPOINT_ARCHIVE:-./yesh-network-endpoint.tar}
trap 'rm -rf -- "$base"' EXIT INT TERM HUP
test ! -e "$archive"

: "${YESH_NETWORK_DNS_NAME:?set YESH_NETWORK_DNS_NAME explicitly}"
: "${YESH_NETWORK_TCP_HOST:?set YESH_NETWORK_TCP_HOST explicitly}"
: "${YESH_NETWORK_TCP_PORT:?set YESH_NETWORK_TCP_PORT explicitly}"
: "${YESH_NETWORK_TIMEOUT_HOST:?set YESH_NETWORK_TIMEOUT_HOST explicitly}"
: "${YESH_NETWORK_TIMEOUT_PORT:?set YESH_NETWORK_TIMEOUT_PORT explicitly}"

"$yesh" --plan "$example" >"$base/plan.out" 2>"$base/plan.err"
test ! -s "$base/plan.err"
grep -F 'PLAN ["getent" "ahosts"' "$base/plan.out" >/dev/null
grep -F 'PLAN ["nc" "-z" "-w" "5"' "$base/plan.out" >/dev/null

"$yesh" "$example" >"$base/positive.out" 2>"$base/positive.err"
test ! -s "$base/positive.err"

fake_bin=$base/fake-bin
mkdir -m 700 "$fake_bin"
trace=$base/nc.trace
printf '%s\n' '#!/bin/sh' 'printf "%s\\n" "$*" >> "$TRACE_FILE"' 'exit 77' >"$fake_bin/nc"
chmod 700 "$fake_bin/nc"
set +e
TRACE_FILE="$trace" PATH="$fake_bin:$PATH" YESH_NETWORK_DNS_NAME=localhost \
    "$yesh" "$example" >"$base/negative-tcp.out" 2>"$base/negative-tcp.err"
tcp_status=$?
set -e
[ "$tcp_status" -ne 0 ]
test -s "$trace"

printf '%s\n' '#!/bin/sh' 'exit 77' >"$fake_bin/getent"
chmod 700 "$fake_bin/getent"
: >"$trace"
set +e
TRACE_FILE="$trace" PATH="$fake_bin:$PATH" YESH_NETWORK_DNS_NAME=localhost \
    "$yesh" "$example" >"$base/negative-dns.out" 2>"$base/negative-dns.err"
dns_status=$?
set -e
[ "$dns_status" -ne 0 ]
test ! -s "$trace"

start=$(date +%s)
set +e
nc -z -w 1 -- "$YESH_NETWORK_TIMEOUT_HOST" "$YESH_NETWORK_TIMEOUT_PORT" >"$base/negative-timeout.out" 2>"$base/negative-timeout.err"
timeout_status=$?
set -e
elapsed=$(( $(date +%s) - start ))
[ "$timeout_status" -ne 0 ]
[ "$elapsed" -ge 1 ]
[ "$elapsed" -le 4 ]

tar -cf "$base/network-endpoint.tar" -C "$base" \
    plan.out plan.err positive.out positive.err \
    negative-tcp.out negative-tcp.err negative-dns.out negative-dns.err \
    negative-timeout.out negative-timeout.err
cp -- "$base/network-endpoint.tar" "$archive"
test -s "$archive"

echo 'RWE endpoint plan, DNS, bounded TCP, ordered negatives, and archive passed'
echo "archive: $archive"
