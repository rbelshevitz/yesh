#!/bin/sh

# Start a bounded systemd service that performs real temporary block I/O.
# The service is removed after it exits and the temporary directory is cleaned
# by the service itself. It does not touch nginx or persistent configuration.

set -eu

unit=${1:-yesh-io-mock.service}
runtime=${2:-90s}

case "$unit" in
    *.service) ;;
    *) echo "unit must end in .service: $unit" >&2; exit 2 ;;
esac

if ! command -v systemd-run >/dev/null 2>&1; then
    echo "systemd-run is required" >&2
    exit 2
fi

if systemctl is-active --quiet "$unit" 2>/dev/null; then
    echo "unit is already active: $unit" >&2
    exit 4
fi

workdir=$(mktemp -d /tmp/yesh-io-mock.XXXXXX)

if ! systemd-run \
    --unit="$unit" \
    --collect \
    --property=IOAccounting=yes \
    --property=RuntimeMaxSec="$runtime" \
    --property=PrivateTmp=no \
    /bin/sh -c '
        set -eu
        trap '\''rm -rf -- "$1"'\'' EXIT HUP INT TERM
        while :; do
            dd if=/dev/zero of="$1/payload" bs=1M count=128 conv=fdatasync status=none
            if ! dd if="$1/payload" of=/dev/null bs=1M iflag=direct status=none 2>/dev/null; then
                dd if="$1/payload" of=/dev/null bs=1M status=none
            fi
        done
    ' mock "$workdir"; then
    rm -rf -- "$workdir"
    exit 1
fi

echo "Started $unit with IOAccounting=yes"
echo "Temporary directory: $workdir"
echo "Runtime limit: $runtime"
echo "Run yesh service resources for $unit now."
