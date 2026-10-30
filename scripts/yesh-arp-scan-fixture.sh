#!/bin/sh
# Deterministic test for the yesh arp-scan example.
# It does not send packets and does not need sudo or CAP_NET_RAW.
# The fake command prints both arp-scan's header and one host row so the test
# proves that a header cannot produce a false positive.
set -eu
umask 077

if test "$#" -ne 2; then
    echo "usage: $0 /path/to/yesh /path/to/arp-scan-by-mac.yesh" >&2
    exit 2
fi
yesh=$1
example=$2
test -x "$yesh"
test -r "$example"
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

base=$(mktemp -d /tmp/yesh-arp-scan-fixture.XXXXXX)
trap 'rm -rf -- "$base"' EXIT INT TERM HUP
fake=$base/arp-scan
# Replace the real scanner with a controlled external executable.
cp "$script_dir/yesh-arp-scan-fixture-fake.sh" "$fake"
chmod 0755 "$fake"

# Positive case: the exact IP and MAC occur on one host row.
env -i PATH=/usr/bin:/bin \
    YESH_ARP_INTERFACE=fixture0 \
    YESH_ARP_NETWORK=192.0.2.17/32 \
    YESH_ARP_IP=192.0.2.17 \
    YESH_ARP_MAC=02:00:00:00:00:17 \
    YESH_ARP_SCAN="$fake" \
    "$yesh" "$example" >"$base/positive.out"
grep -F '192.0.2.17 02:00:00:00:00:17' "$base/positive.out"
grep -F 'MAC found' "$base/positive.out"

set +e
# Negative case: the fake output still contains the host, but the requested
# exact /32 is different, so the yesh pipeline must return code 4.
env -i PATH=/usr/bin:/bin \
    YESH_ARP_INTERFACE=fixture0 \
    YESH_ARP_NETWORK=192.0.2.18/32 \
    YESH_ARP_IP=192.0.2.18 \
    YESH_ARP_MAC=02:00:00:00:00:17 \
    YESH_ARP_SCAN="$fake" \
    "$yesh" "$example" >"$base/negative.out"
status=$?
set -e
test "$status" = 4
grep -F 'MAC was not found' "$base/negative.out"

echo "ARP fixture positive, exact-IP filtering, header rejection, and negative passed"
