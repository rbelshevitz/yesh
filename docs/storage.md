# Storage, Block Devices, and fstab: Design

> Status: read-only device lookup, inspection and mount requirements are implemented
> on Linux, including read-only fstab snapshots. Mount/unmount remain design only.

Disk operations are a separate safety boundary. They must not be reduced to a
string command such as `mount`: a wrong device or mountpoint can damage data.

## Target operations

```yesh
disk = device by uuid "..."
inspect $disk
mount disk at /mnt/backup readonly
require mounted /mnt/backup
unmount /mnt/backup
```

Diagnostics should use explicit system sources: `blkid` for UUID, PARTUUID,
TYPE, and LABEL; `lsblk` for the device tree, mountpoint, and read-only state;
`findmnt` for the actual mount relationship; structured `/etc/fstab` parsing;
and `stat`, `df`, and `du` for size and usage.

## Guardrails

- never mount `/dev/sdX` without confirmed device identity;
- UUID/PARTUUID must resolve unambiguously or the operation stops;
- the mountpoint must be a directory and not unexpectedly occupied;
- verify the expected filesystem before backup;
- `require mounted` fails for a missing mount or wrong source;
- the MVP does not write `/etc/fstab` automatically;
- `mount` and `umount` need timeouts and visible `busy` errors;
- partitioning, `mkfs`, `wipefs`, `dd`, `fdisk`, and `parted` are not built-in
  MVP operations;
- `--plan` and dry-run must not pretend to inspect real state.

## Safe backup sequence

```text
resolve UUID -> inspect -> verify filesystem -> verify mountpoint
-> verify free space -> run backup -> verify result -> report source and target
```

Read-only diagnostics come first. Mutating operations require an explicit
action type, preflight checks, and a negative test using the wrong disk.

## Implemented read-only interface

Check the exact backup mount before writing; a directory on the root filesystem
must not pass merely because its parent is mounted.

```yesh
backup = device by uuid "replace-with-real-uuid"
inspect $backup
require mounted /mnt/backup from $backup filesystem "ext4"
echo "backup preflight passed"
```

Bash comparison (the yesh implementation also rejects ambiguous identities):

```bash
set -euo pipefail
source=$(findfs UUID=replace-with-real-uuid)
findmnt --mountpoint /mnt/backup --source "$source" --types ext4 >/dev/null
```

Rule: `device by uuid|partuuid value` resolves one identity, and
`require mounted path from device filesystem type` requires that exact directory
to be mounted from that device with the specified filesystem.

`inspect device` prints PATH, MAJ:MIN, UUID, PARTUUID, FSTYPE, LABEL and RO as
JSON. Identities are compared exactly, without case folding. A missing match or duplicate
reported identity stops execution; no sudo or device probing is
performed. Unreported/inaccessible metadata cannot prove global uniqueness.
Linux uses explicit JSON columns from `lsblk` and `findmnt`
(util-linux package). Queries have a 10-second execution limit, a two-second cancellation grace
period and a 4 MiB limit per output stream.
`require mounted` resolves the identity again and rejects changed devices,
symlink mountpoints, filesystem subtrees, and non-directories. Whole-filesystem
bind mounts cannot be distinguished from ordinary mounts by this check.

Error example: `/mnt/backup` exists on `/` but is not itself mounted, or the
mounted filesystem has a different UUID/type: execution stops before backup.
Checks are snapshots, not a lock against concurrent mount/device changes and
not proof of future write access. Run without concurrent mount changes.

`--plan` reports deferred lookup, inspection and mount checks without running
utilities or inspecting paths; it does not claim the disk is available.
Device values cannot be interpolated into commands or converted to strings.
Mount/unmount, automatic fstab writing and destructive built-ins are unavailable
through this interface. Ordinary external commands keep their existing policy.

## Reading fstab (Linux)

```yesh
mounts = fstab "/etc/fstab"
inspect $mounts
echo "fstab snapshot read"
```

Bash comparison:

```bash
findmnt --fstab --json --list --nocanonicalize \
  --output SOURCE,TARGET,FSTYPE,OPTIONS,FREQ,PASSNO
```

Rule: `fstab path` reads one bounded regular-file snapshot through util-linux
libmount; `inspect` shows ordered entries and diagnostics, never applies them.
Unlike this bare Bash command, yesh rejects parser warnings even with status 0.
Example error: a broken line followed by a valid entry must stop the script,
not silently yield a partial table. Empty/comment-only files are valid.

Input is limited to 4 MiB, 10,000 entries and 64 KiB per line. No device/tag
resolution, path canonicalization, mount helpers or systemd generator runs.
A private temporary snapshot is removed after the query. Plan defers reading.
Options are retained internally as one string: quoted commas and repeated
options must not be split or sorted. Inspection omits source and option values
for network/other sources and shows only option names, avoiding credential
values. Local paths and UUID/PARTUUID/LABEL/PARTLABEL sources remain visible.
Duplicate non-swap targets are diagnostics, not automatically invalid: preserve
order and investigate intent. Swap entries sharing `none` are not duplicates.

This reads configuration, not active mounts or effective systemd dependencies.
`nofail` permits boot to continue; it does not prove backup storage is mounted.
`noauto` does not disable `x-systemd.automount`. `passno` ordering differs in
systemd, which treats it as a boolean except for root-first checking.
Unknown options remain opaque; no claim of boot validation is made.
Sources: [fstab(5)](https://man7.org/linux/man-pages/man5/fstab.5.html),
[libmount parser](https://www.kernel.org/pub/linux/utils/util-linux/v2.41/libmount-docs/libmount-Table-of-filesystems.html),
[systemd.mount](https://www.freedesktop.org/software/systemd/man/latest/systemd.mount.html),
[systemd-fstab-generator](https://www.freedesktop.org/software/systemd/man/latest/systemd-fstab-generator.html).

## Checking an fstab entry against an active mount

```yesh
mounts = fstab "/etc/fstab"
require mounted /mnt/backup from $mounts
echo "configured backup device is mounted"
```

Bash comparison (additional checks are needed for ambiguity and identity):

```bash
findmnt --fstab --mountpoint /mnt/backup --output SOURCE,FSTYPE
findmnt --kernel --mountpoint /mnt/backup --output SOURCE,FSTYPE,MAJ:MIN
```

Rule: `require mounted path from fstab_snapshot` requires exactly one matching
fstab entry, a unique UUID/PARTUUID device, and the exact active mount with
matching device and filesystem. Missing/duplicate entries, unavailable devices,
wrong filesystems and a plain directory on `/` stop the script before backup.

This first check supports whole local filesystems with an explicit type and
UUID/PARTUUID source (optional enclosing quotes are literal fstab quoting).
Sources by path/LABEL, swap, auto/type lists, bind/rbind, move/remount, loop and
btrfs subvolume options are rejected as unsupported, never guessed.
Lexically equivalent duplicate target paths are ambiguous too. The check uses
the loaded snapshot, not subsequent edits to the original fstab file.

Kernel mount lookup disables canonicalization and precedes filesystem path
inspection; an autofs-only target must fail without activating the automount.
`noauto`/`nofail`/`x-systemd.automount` never excuse an absent mount.
Stacked active mounts are rejected rather than choosing the first match.
Plan defers entry selection, device lookup and actual mount checks.
This verifies target, device and filesystem only: mount options, write access,
free space, boot readiness and concurrent changes are separate checks.

## Read-only mount checks (next storage slice)

The current `require mounted` operation intentionally does not imply that a
backup is writable, read-only, or large enough. These are separate observations
because a mount can be read-only while the directory is accessible, and free
space can change immediately after the check.

Before adding a storage builtin, the Linux test environment must demonstrate:

- effective options from the kernel mount table, not only options copied from
  `/etc/fstab`;
- an explicit read-only requirement and an explicit writable requirement;
- a write probe that uses a uniquely named file in the target directory,
  removes only that file, and reports permission/read-only failures distinctly;
- free-space checks using filesystem statistics with checked binary units;
- a race disclaimer: these are preflight observations, not a lease or a
  guarantee for a later backup operation.

Until these rules have a privileged fixture and negative tests, scripts should
use the existing external `findmnt`, `df`, and `free_space` checks explicitly.
No `mount`, `umount`, `/etc/fstab` write, or implicit write probe is provided.

Network filesystems are a separate diagnostic path. A local UUID/PARTUUID
identity check must never be reused for NFS/CIFS. For `credentials=` yesh may
eventually validate only an external regular file's ownership and restrictive
mode; it must not read, print, or create that file. Inline `password=` remains
an error for any future fstab-writing interface.
