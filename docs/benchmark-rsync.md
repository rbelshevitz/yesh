# Incremental rsync benchmark

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
and a failed/partial transfer. Real mount, UUID/PARTUUID, full-disk, and
production SSH tests are separate environment-dependent cases.

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
