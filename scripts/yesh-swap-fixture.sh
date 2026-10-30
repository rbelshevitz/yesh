#!/bin/sh
# Disposable loop-backed swap fixture. Run as root:
#   sudo sh /home/max/yesh-swap-fixture.sh
#
# It changes only the kernel's active swap list for one private loop device.

set -eu
umask 077

test "$(id -u)" -eq 0
test -x /sbin/losetup
test -x /sbin/mkswap
test -x /sbin/swapon
test -x /sbin/swapoff
test -x /bin/findmnt

base=$(mktemp -d /tmp/yesh-swap.XXXXXX)
image=$base/swap.img
loop=
active=0

cleanup() {
    rc=$?
    if test "$active" -eq 1; then
        /sbin/swapoff "$loop" >/dev/null 2>&1 || rc=1
    fi
    if test -n "$loop"; then
        /sbin/losetup --detach "$loop" >/dev/null 2>&1 || rc=1
    fi
    case "$base" in
        /tmp/yesh-swap.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

truncate -s 32M "$image"
loop=$(/sbin/losetup --find --show --nooverlap "$image")
case "$loop" in
    /dev/loop[0-9]*) ;;
    *) echo "ERROR: unexpected loop device: $loop" >&2; exit 1 ;;
esac

/sbin/mkswap -L "yesh-$$" "$loop" >/dev/null
/sbin/swapon "$loop"
active=1
swap_info=$(/sbin/swapon --show=NAME,TYPE,SIZE,USED,PRIO --noheadings)
printf 'SWAP %s\n' "$swap_info"
case "$swap_info" in
    *"$loop"*partition*|*"$loop"*file*) ;;
    *"$loop"*) ;;
    *) echo "ERROR: disposable loop device is not active swap: $swap_info" >&2; exit 1 ;;
esac

if /bin/findmnt --kernel --source "$loop" >/dev/null 2>&1; then
    echo 'ERROR: swap loop device also appeared as a mount' >&2
    exit 1
fi
/sbin/swapoff "$loop"
active=0
if /sbin/swapon --show=NAME --noheadings | grep -Fq "$loop"; then
    echo 'ERROR: swapoff left disposable loop active' >&2
    exit 1
fi
echo 'swap is separate from mount filesystem; cleanup passed'

