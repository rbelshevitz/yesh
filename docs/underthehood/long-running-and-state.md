# Long-running operations, progress, and state

Status: design for a later layer. Arbitrary timeouts and durable resumption are
not language features yet.

## What works now

An ordinary external command or pipeline has a five-minute limit. It is then
cancelled by stopping its process group and waiting for direct children. This
is not a script-wide budget. `pv`, rsync, and rsnapshot remain external
programs. Streams are forwarded without accumulating all stdout in memory;
stderr remains separate.

```yesh
tar -cf - -- source | pv --numeric --bytes | gzip > partial
mv -- partial archive
```

Bash comparison: the same command needs `set -o pipefail` and careful variable
quoting. Rule: data progress and successful completion are different facts. An
error from `pv`, tar, or gzip stops the pipeline before the archive is
published.

An empty stdin or an unknown total size does not justify promising an exact
percentage or end time. Only processed bytes can be shown. In cron, numeric
stderr is preferable to interactive redraw. Do not put TTY control sequences
in a JSON journal or mix them with the archive. See the
[`pv` manual](https://www.ivarch.com/programs/quickref/pv.shtml).

## Budgets for a later layer

The wait for a lock, operation time, inactivity, total job lifetime, and
resource-release time need separate budgets. No stdout does not necessarily mean
that a command is stuck: compression, `fsync`, or the remote side may be
working silently. A pause in progress must not by itself authorize restarting
an arbitrary command.

The exact syntax is not selected yet; no special variable will secretly change
a timeout. Infinite waiting is not the default. A package transaction, a
systemd job, and snapd work each require their own cancellation method; killing
the CLI does not prove that the manager stopped the underlying work.

A long-running job that can be observed should have a name or ID, an owner, a
cancellation source, a process boundary, and a final state that identifies a
partial result. This belongs to the future job model, not implicit
backgrounding through `&`.

## Three different kinds of data

1. An event journal is a sequence of timestamped events and results. JSON Lines
   (JSONL) with a bounded record size is suitable for machine reading.
2. A state snapshot is a small versioned document describing the current job.
3. A resumption point is a verified application stage whose result is known to
   be safe to reuse.

One JSON file does not automatically provide the third item. A PID without its
start time does not identify the previous process. A process disappearing does
not prove that an archive, package installation, or rotation is complete.

Minimum fields include the format version, job ID, input parameters without
secrets, verified object identifiers, the completed stage, cancellation or
error state, partial-result paths, and their verification data. An unknown
version or inconsistent fields is a refusal, not a guess.

## Writing and reading

The future standard write path is: validate directory and lock, create a
temporary file with restricted permissions, write complete JSON, `fsync` the
file, rename within the same filesystem, synchronize the directory when
supported, and verify again. After an error, the last confirmed stage must be
clear; one rename does not make several files an atomic transaction.

Limit size, nesting, strings, and collections. Distinguish an empty, truncated,
and corrupted file. Define duplicate-key handling: a decoder must not silently
replace a conflicting value. Do not reconstruct commands from state strings or
parse state through a shell again. Secrets and full argv values containing
passwords must not be ordinary state fields.

Links and hard links in the state path need the same checks as configuration
changes. Read-only mode must never repair a corrupted state file automatically.

## Real checks

The current [backup practical example](../practical-examples.md) covers backup with a
JSON report and cancellation without publishing a false success. It is an
application report, not yesh's kernel journal and not a resumption protocol.

Future checks need a failure at every write stage, exhausted space, two
competing runs, a substituted link, a stale PID, changed inputs, and a restart.
Resumption is safe only for operations with defined repeat semantics; there are
no automatic retries for install, mount, or restart.
