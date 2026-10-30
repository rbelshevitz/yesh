#!/bin/sh
# Read-only systemd/fstab mount diagnostics.
# Usage:
#   sudo sh /home/max/yesh-systemd-mount-diagnostics.sh
#
# Does not run daemon-reload, mount, umount, enable, start, or stop.

set -eu
umask 077

test "$(id -u)" -eq 0
test -x /bin/findmnt
test -x /bin/systemctl
test -x /bin/systemd-escape
test -r /etc/fstab
test -r /proc/cmdline

case "$(ps -p 1 -o comm=)" in
    systemd) ;;
    *) echo 'SKIP: PID 1 is not systemd' >&2; exit 77 ;;
esac

echo 'SYSTEMD'
systemctl show --no-pager --property=Version,Features --value 2>/dev/null || true

echo 'INITRD'
if test -r /etc/initrd-release; then
    echo 'present /etc/initrd-release'
else
    echo 'absent /etc/initrd-release'
fi

echo 'KERNEL-CMDLINE'
tr ' ' '
' </proc/cmdline |
    grep -E '^(fstab|rd\.fstab|root|rootfstype|rootflags|mount\.usr|mount\.usrfstype|mount\.usrflags|systemd\.)=' ||
    true

entries=$(findmnt --fstab --noheadings --output TARGET,FSTYPE,OPTIONS 2>/dev/null || true)
echo 'FSTAB-RELEVANT'
printf '%s
' "$entries" |
    while IFS= read -r line; do
        test -n "$line" || continue
        target=$(printf '%s
' "$line" | awk '{print $1}')
        fstype=$(printf '%s
' "$line" | awk '{print $2}')
        options=$(printf '%s
' "$line" | cut -d ' ' -f 3-)
        case ",$options," in
            *,_netdev,*|*,nofail,*|*,x-systemd.automount,*)
                safe_options=$(printf '%s
' "$options" |
                    sed -E 's/(password|passwd|credentials|username|user|passfile)=[^, ]+/\1=<redacted>/Ig')
                printf 'ENTRY target=%s fstype=%s options=%s\n' \
                    "$target" "$fstype" "$safe_options"
                mount_unit=$(systemd-escape --path --suffix=mount "$target")
                automount_unit=$(systemd-escape --path --suffix=automount "$target")
                printf '  MOUNT-UNIT %s\n' "$mount_unit"
                systemctl show --no-pager \
                    --property=LoadState,FragmentPath,SourcePath,After,Wants,Requires \
                    "$mount_unit" 2>&1 | sed 's/^/    /'
                if printf ',%s,' "$options" | grep -Fq ',x-systemd.automount,'; then
                    printf '  AUTOMOUNT-UNIT %s\n' "$automount_unit"
                    systemctl show --no-pager \
                        --property=LoadState,FragmentPath,SourcePath,After,Wants,Requires \
                        "$automount_unit" 2>&1 | sed 's/^/    /'
                fi
                ;;
        esac
    done

echo 'GENERATOR-DIRECTORIES'
for directory in /run/systemd/generator /run/systemd/generator.early /run/systemd/generator.late; do
    if test -d "$directory"; then
        printf '%s\n' "$directory"
        find "$directory" -maxdepth 1 -type f -o -type l |
            sort |
            sed 's/^/  /'
    else
        printf '%s absent\n' "$directory"
    fi
done

echo 'systemd mount dependency diagnostics passed (read-only)'

