# Practical Examples

An example is useful only when its result is checked: data is restored, invalid
configuration does not replace the working file, and an interrupted operation
is not reported as successful. An exit code alone is not sufficient.

All files in `examples/` are checked for parsing. The practical scenarios are
exercised by `cmd/yesh/practical_posix_test.go` and
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
| Static release publication with OpenRC | [`deploy-static.yesh`](../examples/deploy-static.yesh) | missing file, unsafe current path, failed HTTP check, or failed HTTP check after rollback |
| Static release publication with OpenRC/lighttpd | [`deploy-static-openrc-lighttpd.yesh`](../examples/deploy-static-openrc-lighttpd.yesh) | missing file, invalid lighttpd configuration, failed HTTP check, or failed HTTP check after rollback |
| Static release publication with systemd | [`deploy-static-systemd.yesh`](../examples/deploy-static-systemd.yesh) | missing file, unsafe current path, failed HTTP check, or failed HTTP check after rollback |
| Release switching and unconditional rollback | [`deploy-release-rwe.yesh`](../examples/deploy-release-rwe.yesh) | missing source or incomplete staging |
| Read-only incident report | [`incident-report-rwe.yesh`](../examples/incident-report-rwe.yesh) | unavailable service, journal, or diagnostic source |
| Networked rsync snapshot | [`rsync-networked-incremental-rwe.yesh`](../examples/rsync-networked-incremental-rwe.yesh) | DNS, route, SSH, or remote-path failure |
| HTTPS service preflight | [`network-service-preflight.yesh`](../examples/network-service-preflight.yesh) | DNS, route, TLS, or HTTP failure |
| Selected-snapshot restore | [`rsync-selected-restore.yesh`](../examples/rsync-selected-restore.yesh) | symlink source or non-empty target |
| rsnapshot with confirmation | [`rsnapshot-apply.yesh`](../examples/rsnapshot-apply.yesh) | missing confirmation or failed config test |
| Scheduled backup | [`cron-backup-rwe.yesh`](../examples/cron-backup-rwe.yesh) | cancellation, timeout, or held lock |
| nginx text-log maintenance | [`nginx-logrotate-maintenance.yesh`](../examples/nginx-logrotate-maintenance.yesh) | missing confirmation or failed service/logrotate check |
| OpenRC service observation | [`openrc-resource-check.yesh`](../examples/openrc-resource-check.yesh) | missing service, unavailable cgroup, or incomplete measurement |
| OpenRC automatic manager selection | [`openrc-auto-resource-check.yesh`](../examples/openrc-auto-resource-check.yesh) | no confirmed service manager |
| OpenRC `supervise-daemon` child restart | [`openrc-supervise-check.yesh`](../examples/openrc-supervise-check.yesh) | child epoch changes during measurement |

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
directory, then publish it only after `rsync` succeeds. The previous-snapshot
variable must be set; for the first snapshot it may point to a deliberately
nonexistent path. When it exists, `--link-dest` reuses unchanged files from
that snapshot; it does not make the temporary tree or the previous snapshot
writable. The example deliberately does not add `--delete`:

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
When a test changes a source file between runs, change its size or ensure a
new modification time; rsync's normal quick-check uses size and modification
time and can legitimately treat a rapid same-size rewrite as unchanged.

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

### Guarded rsnapshot rotation

[rsnapshot-apply.yesh](../examples/rsnapshot-apply.yesh) is a preflight wrapper,
not a replacement for `rsnapshot.conf`. The configuration remains the source of
truth for `retain`, `sync_first`, `no_create_root`, `lockfile`, backup points,
`rsync_long_args`, and other rsnapshot parameters.

`configtest` checks the configuration syntax and the referenced helper paths;
it does not copy data, prove that the source is readable, prove that the
destination is the intended mounted filesystem, or guarantee that a later
transfer will succeed. yesh performs the mount and free-space checks around it,
while rsnapshot remains responsible for parsing and applying its own options.

The visible order is:

1. verify the configuration, mounted destination, and free space;
2. run `rsnapshot configtest`;
3. capture test output with `rsnapshot -t`;
4. review the report, including `--delete` and `--delete-excluded` flags;
5. create the confirmation file outside yesh;
6. run `sync` first when `sync_first=1`, then run the selected interval;
7. require the expected `daily.0` directory and absence of the lockfile;
8. restore an older snapshot into a separate empty directory.

The `/./` suffix in a `backup` line controls the directory layout: it copies
the contents of the source below the configured destination instead of adding
the source directory name. `daily.0` is the newest completed rotation and
`daily.1` is the previous one. Rotation may reuse unchanged files with hard
links, so verify both content and inode identity when testing space reuse.

`configtest` and `-t` are review steps, not proof that the real transfer will
succeed. A failed or interrupted operation must be checked for its lockfile,
partial output, and published snapshot. With `no_create_root=1`, rsnapshot
must not create a missing mount root; yesh still checks the expected mount
source before invoking it.

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

[`network-service-preflight.yesh`](../examples/network-service-preflight.yesh)
is the broader read-only chain for a service that will be published or switched:
it checks DNS, the route, TLS/SNI, and an HTTPS health response in separate
steps. `getent`, `ip`, `openssl`, and `curl` remain visible external commands;
yesh does not provide a DNS, TLS, or HTTP client. A failed later step does not
turn earlier successes into an overall success, and an unavailable tool is a
failed or unavailable check rather than proof that the network is healthy.

## Static release publication with OpenRC and systemd

[`deploy-static.yesh`](../examples/deploy-static.yesh) and its paired
[`deploy-static-systemd.yesh`](../examples/deploy-static-systemd.yesh) are small
deployment scenarios for a static site served by nginx. The first targets
OpenRC and calls `rc-service nginx status`; the second targets systemd and uses
the typed `service status nginx.service` operation.

Both examples check the prepared release, the previous release, the current
symlink, and private temporary names while holding a lock. `nginx -t`, the
publication commands, and `curl` remain visible.

For the Alpine/OpenRC fixture, use
[`deploy-static-openrc-lighttpd.yesh`](../examples/deploy-static-openrc-lighttpd.yesh).
It validates the supplied `lighttpd` configuration with `lighttpd -tt -f` and
checks `rc-service lighttpd status`; the publication and HTTP boundaries are
the same.

For a disposable positive and rollback check on a systemd host, use
[`scripts/yesh-deploy-static-systemd-fixture.sh`](../scripts/yesh-deploy-static-systemd-fixture.sh).
It creates a unique directory under the configured web root, runs `--plan`,
publishes one release, then forces a failed HTTP check and verifies the return
to the previous release. It requires root only because the example's
`nginx -t` checks the real nginx configuration; it does not restart nginx or
edit that configuration. The same fixture also interrupts a run after
`current` switches and verifies that no automatic rollback is claimed. Set
`YESH_WEB_ROOT` when the site's document root is not `/var/www/html`; set
`KEEP_FIXTURE=1` to inspect the fixture after the run.

### Privileges are an explicit boundary

These examples do not call `sudo` and do not raise privileges. The operator must
choose the privilege boundary before running them:

- run the complete yesh script as `root` on a controlled deployment host; or
- use a dedicated deployment account that owns the release tree and has a
  narrowly scoped, non-interactive external policy for the web-server
  configuration check.

On the Debian systemd fixture, `/usr/sbin/nginx -t` reports valid syntax but
also needs access to `/run/nginx.pid`; an ordinary user can therefore see the
configuration check fail with `Permission denied`. A password-protected
interactive `sudo` is not a valid unattended deployment design. If a policy is
used, it must be prepared outside yesh and must not turn into a hidden password
prompt. The script's lock, publication, rollback, and HTTP check do not prove
that the caller has the required filesystem or service-manager privileges.

`publish` gives the new release its final immutable directory name. The active
`current` name is then switched by `ln -s` followed by `mv -T`, because yesh's
`publish` deliberately refuses to replace an existing destination. The HTTP
check runs after the switch; on failure the script restores the supplied
previous release, performs the same HTTP check again, and exits non-zero. The
second check observes the result of the restoration; it does not decide whether
the restoration is attempted.

The example proves ordering, same-filesystem publication, lock ownership, and
the selected HTTP check on one host. It does not prove DNS, TLS, client caches,
or behavior outside the requested URL. Run `--plan` first and use a disposable
tree; the real positive, failed-health, stale-temporary-link, and missing-file
cases still need a fixture-backed integration test.

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

### OpenRC on Linux (validated on Alpine)

These examples apply the same finite observation model to a Linux system that
uses OpenRC instead of systemd:

- [`openrc-resource-check.yesh`](../examples/openrc-resource-check.yesh) uses
  an explicit OpenRC adapter for status, resources, and a short measurement;
- [`openrc-lighttpd-check.yesh`](../examples/openrc-lighttpd-check.yesh) is the
  clean-service variant used by
  [`scripts/yesh-openrc-lighttpd-fixture`](../scripts/yesh-openrc-lighttpd-fixture)
  so the SSH observer is not part of the measured cgroup;
- [`openrc-lighttpd-summary.yesh`](../examples/openrc-lighttpd-summary.yesh)
  reads that JSONL and prints the human-readable history report;
- [`openrc-lighttpd-load-fixture`](../scripts/yesh-openrc-lighttpd-load-fixture)
  adds a temporary large page and bounded localhost requests, then appends a
  second measurement to the same JSONL so changes can be compared;
- [`openrc-auto-resource-check.yesh`](../examples/openrc-auto-resource-check.yesh)
  lets yesh confirm the available service manager;
- [`openrc-supervise-check.yesh`](../examples/openrc-supervise-check.yesh)
  observes a temporary `supervise-daemon` service and records an incomplete
  measurement if its child process is replaced during the interval;
- [`openrc-missing-service.yesh`](../examples/openrc-missing-service.yesh) is
  the negative case and must not produce a successful service observation.

Start with `--plan`. The OpenRC status query uses `rc-status -C -f ini -a` so
that services from all runlevels, including stopped services, are considered.
The real observation reports the platform and supervisor separately, then
connects the OpenRC state with the verified PID, `/proc`, and cgroup data.
Unavailable cgroup fields remain unavailable; yesh does not turn this into a
permanent monitor or an OpenRC management tool.

The examples were run on an Alpine VM with OpenRC and cgroup v2. The
`lighttpd` fixture produced one process, three JSONL samples, and
`complete=true`; its `partial` observation was caused by unavailable PSI fields,
not by an incomplete process snapshot. Its human-readable summary also
reported stable memory and RSS, CPU time, OOM kills, block I/O, process
continuity, and the `platform=alpine supervisor=openrc` context. That proves the
selected Alpine/kernel setup only. User services requiring `XDG_RUNTIME_DIR`
and containers are separate checks and are not covered by these examples.

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
