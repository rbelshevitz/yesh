# Incremental rsync benchmark

## Topology of the current workflow

**Important:** yesh runs on the backup host, not on the source host. The
backup host is the rsync destination in this workflow. `YESH_RSYNC_ROOT` is a
local directory on that host, and it contains the published snapshots, the
lock, staging directories, and any retention quarantine. The executable
example accepts either a local source or an rsync source such as
`backup-source:/srv/data/`. It does not use `exists $source`: that checks the
local filesystem and cannot validate an SSH or rsync-daemon address. Rsync
itself validates access to and reads from the source.

The current yesh workflow runs on the backup host. The snapshot root is a
local destination directory on that host. This matters because inventory,
locking, publication, and retention inspect and change the destination
filesystem. A remote destination is a separate workflow and is not implied by
the current examples.

This example supports only a destination local to the machine where yesh runs.
Remote destinations are not supported by this workflow.

[`rsync-destination-preflight.yesh`](../examples/rsync-destination-preflight.yesh)
is the read-only first step: it checks the destination mount, type, ownership,
free space, and then reports `KEEP`/`PRUNE` from the actual local root.

This is the first full 100–200 line corpus benchmark for yesh. The source
behavior is taken from the pinned MIT-licensed
[`pedroetb/rsync-incremental-backup`](https://github.com/pedroetb/rsync-incremental-backup)
workflow, but the yesh version must be written independently.

## Disposable Rocky fixture

The benchmark runs without root in a temporary filesystem:

```text
fixture/
  source/                  changing input tree
  snapshots/               published snapshot directories
  state/                    lock and last-success metadata
  restore/                  a new restore target
```

The fixture includes spaces in names, a binary file, a removed source file,
and a failed/partial transfer. Real mount, UUID/PARTUUID, and full-disk tests
are separate environment-dependent cases.

## Confirmed remote run

A real SSH pull was confirmed on a separate Debian machine with `rsync 3.2.7`.
`yesh` ran on the backup host, while the source used the form
`user@host:/tmp/.../data/`. Both sides used disposable directories under
`/tmp`.

Two consecutive runs were checked:

1. the remote tree was pulled into a new temporary snapshot and published;
2. one remote file was changed and another was added, then a second snapshot
   was published with `--link-dest` pointing to the previous snapshot.

The changed and new file contents were verified, the unchanged file retained
the same inode, the temporary staging directory did not remain, and both
temporary trees were removed afterward. This confirms the real
`remote source → local destination` SSH transfer; it does not prove arbitrary
production permissions, key configuration, or network fault tolerance.

The same check is now an opt-in test,
`TestPOSIXRealRsyncPullAndIncremental`. The repository stores no host address;
the test receives `YESH_SSH_TEST_HOST` and `YESH_SSH_TEST_DIR`:

```text
YESH_RSYNC_SSH_INTEGRATION=1 YESH_SSH_TEST_HOST=user@host \
YESH_SSH_TEST_DIR=/tmp/yesh-test \
go test ./cmd/yesh -run TestPOSIXRealRsyncPullAndIncremental -count=1 -v
```

The test requires SSH with `known_hosts` verification, checks remote `rsync`,
creates its own temporary directory, and removes it after completion.

The separate `TestPOSIXRealRsyncRemoteFailureDoesNotPublish` test covers a
remote read failure: a temporary remote file is made unreadable, the new
snapshot is not published, the previous snapshot is unchanged, the incomplete
staging directory remains available for diagnosis, and a retry succeeds after
permissions are restored. This exercises rsync exit code 23, which is a
partial transfer due to an error and is not success.
The same test removes the remote source before a retry; source-selection
failure also cannot publish a snapshot or change the last successful one.

Exit codes 24 and 30/35 are intentionally deferred from the required suite.
Code 24 needs a reproducible race in which a file disappears during reading;
codes 30/35 need a separate controlled delay or connection-drop harness.
Injecting those conditions as a random race would make the test flaky. This is
deferred coverage, not permission to treat those failures as success.

## Required stages

1. Validate source, destination, and an empty/new restore target.
2. Acquire an exclusive lock for the whole operation.
3. Run `rsync` into a temporary snapshot using `--link-dest` only when the
   previous snapshot was positively identified.
4. Publish the snapshot atomically after rsync succeeds.
5. Write state only after publication.
6. Apply explicit retention and show deletions in `--plan`.
7. Restore the selected snapshot into a new directory and compare bytes.

The negative matrix covers missing source, wrong previous snapshot, lock
conflict, rsync failure, interrupted publication, a second run, and restore
verification. A failed run must preserve the previous published snapshot and
must not write a successful state record.

The repository now has a real unprivileged two-run fixture test,
`TestPOSIXPracticalIncrementalRsyncRoundTrip`. It executes the example with
the installed `rsync`, publishes two snapshots, verifies changed and
unchanged files, and checks that `--link-dest` reuses the unchanged inode.
It also covers a rerun, missing-source failure, and restore through external
`rsync`. A separate
`TestPOSIXPracticalIncrementalRsyncPartialFailureDoesNotPublish` test makes
the external `rsync` return code 23 and verifies that the new snapshot is not
published, the previous snapshot is unchanged, staging is closed, and a retry
can acquire the lock. This is a working slice of the benchmark, not
completion of the full negative matrix or retention work. The separate
`TestPOSIXPracticalIncrementalRsyncHeldLockStopsRsync` test holds the advisory
lock and verifies that `rsync` is not started at all.

## What this measures in yesh

The adaptation should make these Bash hazards visible or remove them:

- quoted paths and list arguments remain separate argv values;
- no implicit `set -e` or pipeline-status ambiguity;
- temporary output is not mistaken for a published snapshot;
- `--plan` shows source, destination, `--link-dest`, retention, and lock policy.

The benchmark must also record what remains external. yesh currently has no
dedicated multi-command lock scope, timestamp generator, directory enumeration
API, or rollback/defer construct. Do not add a builtin for one failure until
the same missing capability appears in a second corpus workflow. If an
external helper is used, its exact argv and failure behavior must be tested.

The current language-design hypothesis is deliberately narrower than a
`snapshot` DSL: if the benchmark confirms the gap, consider only generic
cross-workflow primitives such as a scoped exclusive lock and an atomic
publish/rename operation. Both must have explicit failure semantics and be
useful outside backup (for example cron jobs and configuration publication).

`--delay-updates` only makes individual file replacement closer to atomic; it
does not publish a whole directory atomically. Whole-snapshot publication
requires renaming a completed temporary directory on the same filesystem, with
the final name absent or replaced under an explicitly defined policy. The
fixture must reject an existing final or temporary directory rather than
silently merge or delete it.

The durability profile uses explicit `rsync --fsync` when the installed rsync
supports it. This fsyncs each finished file on the receiving side; it does not
by itself fsync the containing directories or prove power-loss durability.
Those stronger guarantees require a separate Linux filesystem test and must
not be inferred from a successful process exit.

Success means a reviewer can explain the state transitions and failure paths
more quickly than in the Bash baseline. A shorter script alone is not success.

## Product target

The useful result is a thin, explicit backup facade: fewer quoting, lock,
publication, and retention mistakes while `rsync`/`rsnapshot` remain visible
and independently documented. A future helper may package the repeated
preflight and state transitions, but it must still expose source, target,
previous snapshot, retention, and destructive actions in `--plan`. It must not
invent a second backup engine, re-export the utilities' growing option sets,
or silently change rsync/rsnapshot semantics. Utility-specific flags stay
ordinary explicit argv arguments.

## Utility practices to preserve

- Run `rsync --link-dest` into a new empty snapshot directory. The previous
  snapshot is a basis, not the destination to update.
- Keep partial files in a private destination-side directory; never use a
  shared world-writable `/tmp` partial directory.
- Treat rsync exit codes as data. In particular, 23/24 mean partial or vanished
  input, and 30/35 indicate timeouts; none may publish a successful snapshot.
- Keep `--delete` explicit. Snapshot creation into a new directory normally
  does not need it; retention deletion is a separate, planned operation.
- For rsnapshot, run `configtest` and its test mode before rotation, and set
  `no_create_root=1` when a removable backup mount must never fall back to a
  local directory.
- Treat `flock` as advisory. Do not claim cross-host serialization on NFS/SMB
  without testing that mounted filesystem's locking mode; provision the parent
  lock directory as trusted because `O_NOFOLLOW` protects only the final path
  component.

## Retention decision

`rsnapshot` normally deletes the oldest interval before continuing rotation;
`use_lazy_deletes` only moves deletion later and may leave temporary
`_delete.*` directories. That is rsnapshot's own contract, not the contract of
this incremental benchmark.

For yesh, use the stricter rule: publish the new snapshot successfully first,
then build and show the exact prune list. Only completed directories with a
narrow, validated name inside the checked root may be removed; source, staging,
lock, symlinks, and the previous snapshot are excluded. A prune failure does
not hide or invalidate an already published backup. Without `--apply`, no
deletion is allowed.

`examples/rsync-retention-plan.yesh` and
`TestPOSIXPracticalIncrementalRsyncRetentionPlanDoesNotMutate` cover only the
preview slice. The `rsync retention plan in $root keep $keep` form computes
candidates in the runtime and does not mutate the filesystem in `--plan`;
staging, lock, symlink, and published snapshots remain unchanged. The future
apply operation must receive the root, retention count, and explicit
current/previous roles; it must refuse to run without them. Applying prune
still requires a separate explicit contract and test.

## Filesystem compatibility profile

The backup contract must name the destination filesystem. `rsync -a` preserves
Unix-oriented metadata (permissions, ownership, symlinks, devices, and special
files) only when the destination can represent it; FAT/exFAT-like targets do
not provide the same model, and ACL/xattr preservation requires compatible
support on both sides. For a portable data copy, use an explicit reduced
profile such as `-rlt`/`--no-perms --no-owner --no-group --no-devices` and
decide separately what to do with links and special files.

Timestamp granularity is also part of correctness. FAT stores timestamps with
coarser resolution; rsync documents `--modify-window=1` for that case.
Case-insensitive or Unicode-normalizing filesystems can collapse distinct
Linux names, so `--delete` can turn a naming collision into data loss. Detect
these properties before a destructive run and keep `--delete` out of the
incremental snapshot path unless the collision policy is explicit.

`--link-dest` depends on hard links and matching preserved attributes. It is
not a universal space-saving mechanism: unsupported link types may be copied,
and a destination filesystem that cannot hard-link regular files loses the
deduplication benefit. Keep the previous snapshot and new temporary snapshot
on the same trusted filesystem and treat a failed link as a profile result,
not as proof that the snapshot is equivalent.

References: the [rsync manual](https://rsync.samba.org/ftp/rsync/rsync.1),
[rsnapshot HOWTO](https://rsnapshot.org/rsnapshot/docs/docbook/rest.html), and
[rsnapshot FAQ](https://rsnapshot.org/faq.html).
