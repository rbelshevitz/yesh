# Practical Examples

An example is useful only when its result is checked: data is restored, invalid
configuration does not replace the working file, and an interrupted operation
is not reported as successful. An exit code alone is not sufficient.

The scenarios are exercised by `cmd/yesh/practical_posix_test.go` and
`cmd/yesh/initrd_linux_test.go`. Tests use temporary directories, do not touch
production services, and do not require a real remote SSH host.

## Exit-code contract

The following convention belongs to the practical examples, not to the yesh
language itself:

| Code | Meaning |
| --- | --- |
| `0` | operation succeeded |
| `1` | general execution error |
| `2` | a source or destination path is missing |
| `3` | a safety check failed, such as a UUID/PARTUUID mismatch |
| `4` | a required resource is unavailable, such as free space |

Examples should use an explicit `exit code`, and tests should check both the
code and diagnostic output. Do not collapse every failure to `1`: callers such
as backup and CI automation need to distinguish an absent path from an unsafe
destination. The language accepts explicit decimal codes from `0` through
`255`; this convention does not change command exit statuses.

## Backup with verified publication

[backup.yesh](../examples/backup.yesh) writes a tar/gzip archive to a partial
name, publishes it only after the pipeline succeeds, and records a small JSON
state file.

```yesh
source = "source"
partial = "backup.tar.gz.partial"
archive = "backup.tar.gz"
tar -cf - -- source | gzip > partial
mv -- partial archive
```

The rule is simple: publish only after every pipeline stage succeeds. Tests
read the archive with Go's tar and gzip packages, compare names and bytes,
including spaces and binary data, and verify that a failed source read leaves
the previous archive and state report unchanged. This is not a complete backup
policy: it does not provide a snapshot of a changing source or durable resume.

## Read-only storage checks

[storage-check.yesh](../examples/storage-check.yesh) verifies that a target is
on a mounted filesystem, then reports its source, free space, and first-level
size distribution. It does not mount, unmount, or modify anything.

```yesh
mountpoint = /backup
if findmnt --target mountpoint:
    findmnt --target mountpoint
    df -P -h -- mountpoint
    du -x -d 1 -h -- mountpoint
else:
    echo "backup mount is missing"
    exit 2
```

`du -x` does not cross filesystem boundaries and `-d 1` bounds the report.
Access errors are not treated as zero usage. The test covers both an existing
and a missing directory; it does not prove a production mount policy.

[storage-device-check.yesh](../examples/storage-device-check.yesh) adds a
read-only report from `findmnt`, `lsblk`, `blkid`, and `stat`. It reports device
metadata but does not choose or mount a disk. The separate
[storage-identity-check.yesh](../examples/storage-identity-check.yesh) checks
an exact `UUID=` or `PARTUUID=` source before a write operation.

## Rsync preflight and safe copy

[rsync-safe-copy.yesh](../examples/rsync-safe-copy.yesh) checks the source and
destination directories, the exact filesystem identity, and free space before
running rsync. If the identity is wrong, rsync is not started.

```yesh
source = "source/"
target = "target/"
expected_source = environment YESH_BACKUP_SOURCE

if findmnt --source expected_source --target target:
    if free_space target at_least 20GiB:
        rsync -a --itemize-changes -- source target
    else:
        echo "not enough free space"
        exit 4
else:
    echo "target filesystem identity does not match"
    exit 3
```

The expected source comes from checked configuration or the environment, not
from a real identifier embedded in the example. An ordinary
[rsync-copy.yesh](../examples/rsync-copy.yesh) demonstrates copying into an
already selected directory; it does not claim to identify the correct disk.

[rsync-preview.yesh](../examples/rsync-preview.yesh) performs the same guarded
checks with `--dry-run`. The test verifies that previewing does not copy data,
that copying preserves the source, and that extra destination files are not
deleted implicitly.

## Configuration and remote operations

[config-apply.yesh](../examples/config-apply.yesh) validates a staged
configuration before installing and publishing it. An invalid staged file
does not replace the active file; the test checks content and final mode.

[ssh-health.yesh](../examples/ssh-health.yesh) and
[scp-copy.yesh](../examples/scp-copy.yesh) collect the port, timeout, batch
mode, and host-key policy in an explicit argument list. The SCP example checks
the local source before starting `scp`. Tests use recording fixtures to verify
the exact argv and call order; they do not store keys or passwords.

## Read-only process and service checks

[process-snapshot.yesh](../examples/process-snapshot.yesh) finds Linux
processes by exact kernel name and displays selected fields:

```yesh
nginx = processes where name is "nginx"
show $nginx fields pid uid state rss cgroup
summarize $nginx memory rss
```

The snapshot is read-only. It does not send signals or modify processes. RSS
totals are estimates of resident memory, not unique physical memory.

All three process examples were also run directly on Rocky Linux 9.8. The
host-side run found five nginx processes in
`/system.slice/nginx.service`: `PROCESS_COUNT=5`, `RSS_TOTAL=15.46MiB`
(`16211968` bytes), `RSS_MAX=3.37MiB`, and `RSS_AVERAGE=3.09MiB`. This also
confirmed UID, state, and systemd cgroup reporting. Fixture-based tests
separately cover exact-name matching, snapshot fields, RSS aggregation, and
bounded `/proc`.

[service-check.yesh](../examples/service-check.yesh) uses ordinary `systemctl`
commands and stops after an inactive service. The fixtures verify argv, order,
and that later commands do not run. This does not test a real systemd bus.

The typed examples make the higher-value contracts explicit:
[service-status.yesh](../examples/service-status.yesh) reads a fixed property
allowlist, [service-restart.yesh](../examples/service-restart.yesh) verifies
the exact returned unit and `ActiveState` after restart,
[service-wait.yesh](../examples/service-wait.yesh) polls without changing the
unit until a finite deadline and reports the last state on timeout, and
[service-journal.yesh](../examples/service-journal.yesh) bounds time, record
count and output size while requiring valid JSON and clean stderr. Their
fixture test covers exact argv, successful status/restart/wait, wait timeout,
failed postcondition, JSON journal output, and `--plan` with no external
invocation. A real systemd
bus or journal is still an opt-in host check, not proven by the fixture.

For that check, use
[`scripts/yesh-fixture-test.sh`](../scripts/yesh-fixture-test.sh). It uses
unique temporary names only in `/run/systemd/system`, refuses to overwrite an
existing file, removes the fixture through `trap`, and also checks three
negative cases: `oneshot` without `RemainAfterExit`, a timeout while the unit
is `activating`, and a failed restart with `Result=exit-code` and a non-zero
`ExecMainStatus`.

An opt-in run was also completed on a separate Linux test host with systemd
252: `service-status.yesh` and `service-wait.yesh` confirmed
`systemd-journald.service` as `loaded/active/running`, while
`service-journal.yesh` correctly returned an empty successful result.
`service-restart.yesh` was run only with `--plan`; no real restart was made.
The manager reported `degraded`, so this proves the available systemd/journal
path, not overall host health.

The important failure distinction is deliberate: an inactive service after a
successful `systemctl restart` is a failed yesh operation; an empty journal
query is successful and means that no matching records were returned; a
non-empty diagnostic or invalid JSON is an error rather than a partial report.

## Dependencies and practical test run

The example files are executable, but not all are turnkey on an arbitrary
host. Files whose header says **environment-dependent** need the stated local
service, mount, process, SSH, or configuration. Files marked
**fixture-dependent** expect the test fixture described in this document.
Replace placeholders before using them against a real system; `web-1`,
`/backup`, and example UUIDs are not live values.

| Group | Examples | Required context |
| --- | --- | --- |
| Self-contained/fixture-tested | `hello`, `backup`, `config-apply`, `rsync-*`, `initrd-module-check` | temporary files or the documented fixture |
| Host read-only checks | `process-*`, `mount-check`, `storage-*`, `fstab-*`, `journal-report`, `service-check` | Linux `/proc`, mount/fstab, journal/systemd, or a real mounted path |
| Remote/configured | `ssh-health`, `scp-copy`, `deploy`, `rsnapshot-local` | SSH trust/agent, remote target, or prepared config |

The practical Linux suite uses ordinary tools including `tar`, `gzip`, `grep`,
`install`, `mv`, `findmnt`, `rsync`, `pv`, and `rsnapshot`. Linux CI installs
the optional packages explicitly and marks scenarios as skipped when a tool is
absent rather than pretending they passed.

```bash
export GOCACHE="$PWD/.gocache"
export GOMODCACHE="$PWD/.gomodcache"
go test -v ./cmd/yesh -run TestPOSIXPractical -timeout=90s
```

These tests demonstrate real command integration in a controlled fixture;
they do not prove backup durability, a production mount policy, kernel-module
loading, or the availability of a real remote host.
