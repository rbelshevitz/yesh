#!/bin/sh
# Run the active scanner through yesh for one explicit IPv4 address.
# This wrapper validates inputs and never invokes sudo by itself.
set -eu
umask 077

if test "$#" -ne 5; then
    echo "usage: $0 /path/to/yesh /path/to/arp-scan-by-mac.yesh interface ipv4 mac" >&2
    exit 2
fi

yesh=$1
example=$2
interface=$3
address=$4
mac=$5

test -x "$yesh"
test -r "$example"
test -n "$interface"

case "$address" in
    *[!0-9.]*|*.*.*.*.*|.*|*.)
        echo "error: expected a dotted-decimal IPv4 address" >&2
        exit 2
        ;;
esac

old_ifs=$IFS
IFS=.
set -- $address
IFS=$old_ifs
test "$#" -eq 4 || {
    echo "error: expected a dotted-decimal IPv4 address" >&2
    exit 2
}
for octet in "$@"; do
    test "$octet" -ge 0 2>/dev/null && test "$octet" -le 255 2>/dev/null || {
        echo "error: IPv4 octets must be between 0 and 255" >&2
        exit 2
    }
done

case "$interface" in
    *[!A-Za-z0-9_.:-]*)
        echo "error: interface contains unsupported characters" >&2
        exit 2
        ;;
esac

if test -x /usr/sbin/ip && test -x /usr/bin/grep; then
    if /usr/sbin/ip -4 -o addr show dev "$interface" 2>/dev/null |
        /usr/bin/grep -F " inet $address/" >/dev/null 2>&1; then
        echo "error: target IP belongs to the scanning host; run from another host on the same L2 network" >&2
        exit 2
    fi
fi

case "$mac" in
    ??\:??\:??\:??\:??\:??) ;;
    *)
        echo "error: YESH_ARP_MAC must be six colon-separated octets" >&2
        exit 2
        ;;
esac

# Keep /usr/sbin visible for Debian/Ubuntu and Rocky/Alma installations.
# arp-scan may still require the caller to provide CAP_NET_RAW or sudo.
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export PATH
YESH_ARP_INTERFACE=$interface
YESH_ARP_NETWORK=$address/32
YESH_ARP_MAC=$mac
YESH_ARP_SCAN=${YESH_ARP_SCAN:-/usr/sbin/arp-scan}
YESH_ARP_IP=$address
export YESH_ARP_INTERFACE YESH_ARP_NETWORK YESH_ARP_MAC YESH_ARP_SCAN YESH_ARP_IP

exec "$yesh" "$example"
