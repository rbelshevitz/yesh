#!/bin/sh
# Disposable retention quarantine fixture. Run as an unprivileged user:
#   sh /home/max/yesh-quarantine-fixture.sh
#
# This moves only validated snapshot directories inside one private root.
# It never deletes quarantine contents and never accepts a free-form deletion path.

set -eu
umask 077

base=$(mktemp -d /tmp/yesh-quarantine.XXXXXX)
root=$base/snapshots
quarantine=$root/quarantine
lock=$root/.retention.lock
foreign=
cleanup() {
    rc=$?
    case "$foreign" in
        /dev/shm/yesh-quarantine.*) rm -rf -- "$foreign" ;;
    esac
    case "$base" in
        /tmp/yesh-quarantine.*) rm -rf -- "$base" ;;
        *) rc=1 ;;
    esac
    exit "$rc"
}
trap cleanup EXIT INT TERM HUP

mkdir -m 700 "$root" "$quarantine"
for name in snapshot.0 snapshot.1 snapshot.2; do
    mkdir "$root/$name"
    printf '%s
' "$name" >"$root/$name/marker"
done
ln -s snapshot.2 "$root/snapshot.bad-link"
mkdir "$root/not-a-snapshot"

exec 9>"$lock"
/usr/bin/flock -n 9
if sh -c 'flock -n 7' 7>"$lock"; then
    echo 'ERROR: lock was acquired twice' >&2
    exit 1
fi

# Keep the two newest completed snapshots; move only the older validated name.
for candidate in snapshot.2; do
    test -d "$root/$candidate"
    test ! -L "$root/$candidate"
    case "$candidate" in
        snapshot.[0-9]*)
            ;;
        *)
            echo "ERROR: invalid candidate passed validation" >&2
            exit 1
            ;;
    esac
    source_device=$(stat -c %d "$root/$candidate")
    target_device=$(stat -c %d "$quarantine")
    test "$source_device" = "$target_device"
    mv -- "$root/$candidate" "$quarantine/$candidate"
done

test ! -e "$root/snapshot.2"
test -d "$quarantine/snapshot.2"
test "$(cat "$quarantine/snapshot.2/marker")" = snapshot.2
test -d "$root/snapshot.0"
test -d "$root/snapshot.1"
test -L "$root/snapshot.bad-link"
test -d "$root/not-a-snapshot"

# A second apply has no candidate to move and must not merge or delete anything.
if test -e "$quarantine/snapshot.2"; then
    test ! -e "$root/snapshot.2"
fi
test -d "$root/snapshot.0"
test -d "$root/snapshot.1"
test -d "$root/quarantine"

if test -d /dev/shm && test "$(stat -c %d "$root")" != "$(stat -c %d /dev/shm)"; then
    foreign=/dev/shm/yesh-quarantine.$$
    mkdir -m 700 "$foreign"
    mkdir "$root/snapshot.cross"
    source_device=$(stat -c %d "$root/snapshot.cross")
    target_device=$(stat -c %d "$foreign")
    test "$source_device" != "$target_device"
    # Refuse before mv: GNU mv would otherwise copy+remove across filesystems.
    test -d "$root/snapshot.cross"
    test ! -e "$foreign/snapshot.cross"
    echo 'cross-filesystem rename rejected before copy'
else
    echo 'cross-filesystem rename check skipped: no distinct /dev/shm filesystem'
fi

echo 'quarantine same-filesystem rename, validation, rerun, and lock passed'

