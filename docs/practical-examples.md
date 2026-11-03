# Practical Examples

An example is useful only when its result is checked: data is restored, invalid
configuration does not replace the working file, and an interrupted operation
is not reported as successful. An exit code alone is not sufficient.

The scenarios are exercised by `cmd/yesh/practical_posix_test.go` and
`cmd/yesh/initrd_linux_test.go`. Tests use temporary directories, do not touch
production services, and do not require a real remote SSH host.

## How to read an example

Files in `examples/` are runnable material, not pseudocode. The header comments
state the goal, inputs, action order, writes, failure behavior, and the boundary
of what the example proves. Values read with `environment NAME` must be set by
the caller; they have no built-in defaults.

Before an operation that changes state, inspect its plan:

```text
./yesh --plan examples/backup-mounted-rwe.yesh
```

`--plan` shows expanded arguments and branches without running external commands
or opening redirections. For a practical example, check the resulting fixture,
not only the exit code: verify published data, preserved old state on failure,
partial files, and lock cleanup.

## Scenario index

| Task | Example | Main negative case |
| --- | --- | --- |
| Archive published after success | [`backup.yesh`](../examples/backup.yesh) | missing source or failed `tar`/`gzip` |
| Backup to a verified filesystem | [`backup-mounted-rwe.yesh`](../examples/backup-mounted-rwe.yesh) | wrong UUID, unmounted filesystem, low space |
| Configuration publication and restart | [`config-systemd-rwe.yesh`](../examples/config-systemd-rwe.yesh) | validator rejects the configuration |
| Release deployment with rollback | [`deploy-release-rwe.yesh`](../examples/deploy-release-rwe.yesh) | missing source or incomplete staging |
| Read-only incident report | [`incident-report-rwe.yesh`](../examples/incident-report-rwe.yesh) | unavailable service, journal, or diagnostic source |
| Networked rsync snapshot | [`rsync-networked-incremental-rwe.yesh`](../examples/rsync-networked-incremental-rwe.yesh) | DNS, route, SSH, or remote-path failure |
| Selected-snapshot restore | [`rsync-selected-restore.yesh`](../examples/rsync-selected-restore.yesh) | symlink source or non-empty target |
| rsnapshot with confirmation | [`rsnapshot-apply.yesh`](../examples/rsnapshot-apply.yesh) | missing confirmation or failed config test |
| Scheduled backup | [`cron-backup-rwe.yesh`](../examples/cron-backup-rwe.yesh) | cancellation, timeout, or held lock |
| nginx text-log maintenance | [`nginx-logrotate-maintenance.yesh`](../examples/nginx-logrotate-maintenance.yesh) | missing confirmation or failed service/logrotate check |

## Shared rule: exit codes

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

## Practical scenarios

The chapters below follow the same reading pattern: the operator problem,
yesh's visible order, the external command that remains responsible for the
domain operation, and the failure boundary proved by the fixture or host check.

## nginx text logs: logrotate and journald stay separate

### What problem does yesh solve?

It prevents one maintenance script from silently mixing journald and
`logrotate`, or from changing logs without a reviewed plan, confirmation, and
post-operation service check.

The nginx service has two independent message streams. Service output is read
from `systemd-journald`, while `/var/log/nginx/*.log` is managed by the active
`logrotate` policy. The yesh scenario does not discover ownership, delete files
itself, or restart nginx. It checks the service and disk space, runs
`logrotate -d -v`, requires an operator-created confirmation file, applies the
same policy and state file, and finally reads a bounded journal window.

The central order is visible in the script:

```yesh
with lock $lock:
    service status nginx.service
    df -P -h -- /var/log
    $binary -d -v -s $state $config

    if exists $confirm:
        $binary -s $state $config
    else:
        echo "nginx logrotate plan reviewed; confirmation file is missing"
        exit 5

    df -P -h -- /var/log
    service status nginx.service
    journal nginx.service since 5m lines 40
```

The final `journal` query is observation only; it does not rotate or vacuum
journald. Journal cleanup is a separate operation using `journalctl --rotate`
and `--vacuum-*`, with its own plan and confirmation. The complete script and
the Debian verification record are linked from the example and the [Linux
integration evidence](underthehood/linux-integration.md).

## Backup with verified publication

### What problem does yesh solve?

It prevents a failed `tar`/`gzip` pipeline from replacing the last good archive:
the partial file stays private until every stage succeeds.

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

### What problem does yesh solve?

It prevents an operator from treating an unmounted or unexpected filesystem as
the backup destination while keeping the check read-only.

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

### What problem does yesh solve?

It prevents `rsync` from starting against the wrong filesystem or insufficient
destination space; the source, destination, and identity checks are visible
before the external copy.

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

### Incremental snapshot and restore

For a backup rather than a mirror, write each new tree to a fresh temporary
directory, then publish it only after `rsync` succeeds. `--link-dest` reuses
unchanged files from the previous published snapshot; it does not make the
temporary tree or the previous snapshot writable. The example deliberately
does not add `--delete`:

```yesh
source = environment YESH_RSYNC_SOURCE
previous = environment YESH_RSYNC_PREVIOUS
temporary = environment YESH_RSYNC_TEMPORARY
current = environment YESH_RSYNC_CURRENT

if exists $previous:
    rsync -a --dry-run --itemize-changes --link-dest $previous -- $source $temporary
    rsync -a --fsync --delay-updates --link-dest $previous -- $source $temporary
else:
    rsync -a --dry-run --itemize-changes -- $source $temporary
    rsync -a --fsync --delay-updates -- $source $temporary
publish $temporary to $current
```

The production example adds the lock and refuses an existing temporary or
published path; see [rsync-incremental.yesh](../examples/rsync-incremental.yesh).
👀 The first command is a reviewable preview, not a proof that a later copy will
succeed. After the copy, restore the selected completed snapshot into a new
empty directory and compare expected files or checksums. A successful `rsync`
exit code alone is not proof that the application can use the restored data.

The equivalent Bash outline is shorter but leaves the staging and failure
rules to surrounding shell code:

```bash
rsync -a --dry-run --itemize-changes --link-dest "$previous" -- "$source" "$temporary"
rsync -a --fsync --delay-updates --link-dest "$previous" -- "$source" "$temporary"
mv -- "$temporary" "$current"
rsync -a -- "$current/" "$restore/"
sha256sum "$current/file" "$restore/file"
```

The `mv` is safe as a publication boundary only when both paths are on the
same filesystem and the temporary tree is private. For a selected restore,
[rsync-selected-restore.yesh](../examples/rsync-selected-restore.yesh) checks
that the source is a real directory and the target is an existing empty
mounted directory; it never deletes target entries implicitly.

## Configuration and remote operations

### What problem does yesh solve?

It keeps staged publication and SSH/SCP arguments reviewable, so an invalid
configuration does not replace the active file and a missing key does not turn
into a hidden password workflow.

[config-apply.yesh](../examples/config-apply.yesh) validates a staged
configuration before installing and publishing it. An invalid staged file
does not replace the active file; the test checks content and final mode.

[ssh-health.yesh](../examples/ssh-health.yesh) and
[scp-copy.yesh](../examples/scp-copy.yesh) collect the port, timeout, batch
mode, and host-key policy in an explicit argument list. The SCP example checks
the local source before starting `scp`. Tests use recording fixtures to verify
the exact argv and call order; they do not store keys or passwords.

👀 For a remote backup, read these examples in order: `ssh-health` checks that the
user can enter the host without a password prompt; `rsync-network-preflight`
then checks DNS, the route, and the remote source directory before a dry run;
`rsync-networked-incremental-rwe` finally transfers into a private local
staging directory and publishes it only after `rsync` succeeds. These checks do
not establish that the remote files are correct or that the host is trusted:
the operator must verify the host fingerprint and prepare `known_hosts` first.
See [the SSH key section](syntax-guide.ru.md#ssh-ключи-для-источника-на-другом-узле)
for that setup.

## Read-only process and service checks

### What problem does yesh solve?

It gives a bounded, read-only view of processes and services without implying
that a process snapshot explains the cause of load or permits changing the
service.

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

The typed examples make the stronger guarantees explicit:
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
`service-restart.yesh` was run only with `--plan`; no real production restart
was made. Its direct command names the disposable unit used by the fixture;
the fixture script creates and removes that unit in `/run`.
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
| Self-contained/fixture-tested | `hello`, `backup`, `backup-with-progress`, `config-apply`, `rsync-*`, `initrd-module-check` | temporary files or the documented fixture |
| Host read-only checks | `process-*`, `mount-check`, `storage-*`, `fstab-*`, `journal-report`, `service-check` | Linux `/proc`, mount/fstab, journal/systemd, or a real mounted path |
| Remote/configured | `ssh-health`, `scp-copy`, `deploy`, `rsnapshot-local` | SSH trust/agent, remote target, or prepared config |

The `backup-with-progress.yesh` example takes an explicit `YESH_PV` path. If
that file is missing or not executable, yesh stops before starting the archive
pipeline.

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
