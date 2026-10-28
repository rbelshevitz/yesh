# Storage, Block Devices, and fstab: Design

Disk operations are a separate safety boundary. They must not be reduced to a
string command such as `mount`: a wrong device or mountpoint can damage data.

## Target operations

```yesh
disk = device by uuid "..."
inspect disk
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
