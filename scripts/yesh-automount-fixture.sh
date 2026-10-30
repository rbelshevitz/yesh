#!/bin/sh
# Run on a disposable Linux host as root:
#   sudo sh /home/max/yesh-automount-fixture.sh
#
# The fixture creates only a private loop image and transient systemd units.
# It does not edit /etc/fstab, enable units, or touch an existing mount.

set -eu
umask 077

test "$(id -u)" -eq 0
test -x /sbin/mkfs.ext4
test -x /bin/mount
test -x /bin/findmnt
test -x /bin/systemctl
test -x /bin/systemd-escape

case "$(ps -p 1 -o comm=)" in
    systemd) ;;
    *) echo 'SKIP: PID 1 is not systemd' >&2; exit 77 ;;
esac

base=$(mktemp -d /tmp/yesh-automount.XXXXXX)
image=$base/fs.img
mountpoint=/run/yesh-automount-$$
mount_unit=$(systemd-escape --path --suffix=mount "$mountpoint")
automount_unit=$(systemd-escape --path --suffix=automount "$mountpoint")
mount_file=/run/systemd/system/$mount_unit
automount_file=/run/systemd/system/$automount_unit
started=0

cleanup() {
    rc=$?
    if test "$started" -eq 1; then
        systemctl stop "$automount_unit" >/dev/null 2>&1 || rc=1
        systemctl stop "$mount_unit" >/dev/null 2>&1 || rc=1
    fi
    rm -f -- "$automount_file" "$mount_file"
    systemctl daemon-reload >/dev/null 2>&1 || rc=1
    if findmnt --kernel --mountpoint "$mountpoint" >/dev/null 2>&1; then
        umount -- "$mountpoint" >/dev/null 2>&1 || rc=1
    fi
    case "$mountpoint" in
        /run/yesh-automount-*) rmdir -- "$mountpoint" 2>/dev/null || true ;;
        *) rc=1 ;;
    esac
    case "$base" in
        /tmp/yesh-automount.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$mountpoint"
truncate -s 64M "$image"
/sbin/mkfs.ext4 -F "$image" >/dev/null

if test -e "$mount_file" || test -e "$automount_file"; then
    echo 'ERROR: generated unit path already exists' >&2
    exit 1
fi

cat >"$mount_file" <<EOF
[Unit]
Description=yesh disposable automount mount

[Mount]
What=$image
Where=$mountpoint
Type=ext4
Options=loop,ro,noload
EOF

cat >"$automount_file" <<EOF
[Unit]
Description=yesh disposable automount

[Automount]
Where=$mountpoint
TimeoutIdleSec=20s

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl start "$automount_unit"
started=1

automount_info=$(findmnt --kernel --mountpoint "$mountpoint" \
    --output TARGET,SOURCE,FSTYPE,VFS-OPTIONS,FS-OPTIONS --noheadings)
printf 'AUTOMOUNT %s\n' "$automount_info"
case "$automount_info" in
    *autofs*) ;;
    *) echo "ERROR: automount target is not autofs before access: $automount_info" >&2; exit 1 ;;
esac

test -e "$mountpoint/lost+found"
systemctl is-active --quiet "$mount_unit"
mount_info=$(findmnt --kernel --mountpoint "$mountpoint" \
    --types ext4 \
    --output TARGET,SOURCE,FSTYPE,VFS-OPTIONS,FS-OPTIONS --noheadings)
printf 'MOUNTED %s\n' "$mount_info"
case "$mount_info" in
    *ext4*ro*norecovery*) ;;
    *) echo "ERROR: triggered mount is not readonly ext4,norecovery: $mount_info" >&2; exit 1 ;;
esac

if touch "$mountpoint/must-fail" 2>/dev/null; then
    echo 'ERROR: readonly automount accepted a write' >&2
    exit 1
fi
systemctl stop "$automount_unit"
started=0
rm -f -- "$automount_file" "$mount_file"
systemctl daemon-reload
echo 'automount fixture passed'

