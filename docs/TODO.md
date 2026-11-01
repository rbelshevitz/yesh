# Current yesh roadmap

This is the short working map. Historical decisions and completed checks are
kept in the [archive](TODO-history.md); this file is not a promise to implement
every item listed below.

## Current position

yesh is a readable language for sequential Linux automation. External Linux
commands remain visible; yesh makes arguments, ordering, preconditions,
publication boundaries, and failure behavior easier to review.

Confirmed:

- assignments, strings, lists, `if`, `for`, commands, pipelines, redirects,
  `--plan`, `exists`, regular expressions, and explicit `exit code`;
- direct Go-runtime process execution, bounded commands, cancellation, and
  child-process cleanup;
- advisory locks and same-filesystem `publish` without copy fallback;
- practical scenarios around `tar`, `gzip`, `pv`, `rsync`, `rsnapshot`, SSH/SCP,
  systemd, journals, storage, `/proc`, and read-only network observation;
- Linux CI, WSL2, separate Debian fixtures, Rocky Linux checks, and loopback
  ext4 scenarios;
- more than 200 automated tests and four fuzz targets.

The first public development release, `v0.1.0`, is published on GitHub for
Linux amd64 with the binary and checksums. ARM64 is not part of the release
until the binary has been run on real ARM64 hardware or an ARM64 virtual
machine.

## Target: `v0.2.0`

`v0.2.0` is the first substantive feature release after the development
prototype. It adds practical read-only service and resource observation while
keeping the language small. Its acceptance boundary is Linux amd64:

- one clear backup/restore workflow with verification, locking, rerun, failure,
  and `--plan` coverage;
- one clear `rsync`/`rsnapshot` workflow with visible external arguments and
  explicit limits;
- one clear logrotate/journald workflow with separate ownership and no hidden
  duplicate rotation;
- configuration publication followed by service state and bounded journal
  checks;
- a documented `service resources` snapshot for systemd and cgroup v2;
- finite service measurement with bounded history and `summarize`, including
  explicit unit/invocation/cgroup identity and incomplete observations;
- process and cgroup evidence covering RSS, CPU, memory, I/O, PSI, restarts,
  and nested service cgroups without claiming application ownership;
- consistent English examples, Russian explanations, manuals, and links;
- reproducible Linux checks, release build, checksum generation, and a clear
  list of Linux-only and experimental resource-observation limits.

The following are explicitly outside the `v0.2.0` boundary: a new general
monitoring subsystem, durable resume and long-running job identity, direct
cgroup management, threaded-cgroup support, ARM64 support, and a D-Bus or
cgroup library unless a concrete release blocker requires one. New syntax is
allowed only when one of these workflows cannot be made clear with the existing
language and the new rule has parser, runtime, plan, positive, and negative
coverage.

## Next after the pause

The next milestone is polish of the implemented core. Do not expand the syntax
or start `observe`, `resume`, or new typed modules until rough edges in the
code, tests, examples, and manuals have been addressed.

### Recorded decision for log maintenance

For ordinary text logs, the main practical path is a visible `logrotate` call
with its actual configuration and state file, as in
[`nginx-logrotate-maintenance.yesh`](../examples/nginx-logrotate-maintenance.yesh).
Direct `find` is not the general log-maintenance mechanism: it is acceptable
only for an explicitly dedicated application-file directory and must be
described as file cleanup, not as a replacement for `logrotate`.

Use `journalctl` for journald, visible `logrotate` for files covered by its
policy, and direct `find` only for a separate, explicitly owned text-file
directory. Mixed journald and text-file output requires separate operations;
one yesh flag or lock cannot coordinate the two owners. Do not hide an
ownership detector, duplicate rotation, compression, removal, or `postrotate`,
or restart a service merely because files were removed. Treat `copytruncate` as
an explicit fallback with a warning about possible record loss.

Use a documented reload/reopen action for text logs; journald does not need a
service reopen. Ambiguous ownership must fail, and each operation needs
positive, negative, and `--plan` coverage.

Do not try to prove arbitrary text-path ownership with a hidden logrotate
detector: configuration is assembled through includes, patterns, `olddir`, and
multiple files, while applications may open additional logs outside systemd's
`StandardOutput=`/`StandardError=` settings. Implement only finite boundary
checks: reject `/run/log/journal` and `/var/log/journal`, check the explicitly
named directory, owner, and mode, and treat incomplete evidence as refusal.
For files from a logrotate policy, run visible `logrotate` and do not duplicate
its rotation, compression, removal, or `postrotate` actions. The open work is
polishing the verified scenario, its messages, documentation, and negative
fixtures—not adding a new language operation or syntax construct.

## Main direction

Focus on safe, repeatable maintenance operations on one Linux machine. These
are one problem family, not three separate products:

1. backup → result verification → restore;
2. log maintenance → free-space check → service;
3. configuration publication → restart → state verification.

Keep the practical sequence explicit in each workflow:

- logrotate maintenance: validate the policy and available space, run its
  preview, require explicit confirmation, apply it, then recheck space, service
  state, and the bounded journal;
- backup: acquire the lock, validate source and destination, copy to a
  temporary tree, verify the result, publish the completed tree, and test
  restoration;
- configuration: validate the candidate, prepare it separately, publish the
  completed result atomically, restart the service, wait for `active`, and
  inspect its state and journal.

First bring backup with result verification and locking to an exemplary state:
partial failure, rerun, `--plan`, restore, and no false success. Then polish
file cleanup and configuration publication. Do not plan new directives or new
domain modules until this corpus is complete.

1. Consolidate three canonical single-host workflows: backup/restore,
   configuration publication with a service postcondition, and incident
   diagnostics.
2. Optionally prepare a production-like rsnapshot/storage scenario on a
   separate filesystem if a concrete question remains that the existing Linux
   checks do not answer; keep durability limits explicit.
3. Run an ARM64 smoke test and decide whether to add ARM64 to a later release;
   use the [ARM64 plan](arm64.md).
4. Design a separate external-command policy with explicit `allowed`, `denied`,
   and `unknown` outcomes; `allowed` means policy match, not execution success.
5. Consider controlled `PATH` and environment policy for privileged execution.
6. Consider durable job identity and resume rules only after a concrete use case.

The networked `rsync` preflight, partial-transfer, failure, and retry scenarios
have already been exercised on the disposable Debian stand. Do not add a new
network language layer or repeat those checks without a specific new failure.

## Further work: process and resource diagnostics

The bounded `measure` and `summarize` first slice is implemented and described
in the decision archive and the [systemd guide](systemd.md). Further work is
bounded read-only process and cgroup diagnostics: explicit observation scope, PID plus Linux start time,
process membership versus semantic ownership, resource counters, and cleanup
evidence for yesh-owned jobs.

The first practical question is bounded service-memory diagnosis: why was one
exact systemd service OOM-killed, and did it hit a cgroup limit or coincide
with host memory pressure? The read-only report should connect the exact unit,
its observed cgroup scope and descendants, `memory.current`, the applicable
limit, local and hierarchical `memory.events`, selected `memory.stat` fields,
the process table with PID plus start time and RSS/PSS where available, and a
bounded journal window. It must distinguish observation from causation and
report unavailable or incomplete data explicitly.

The companion practical question is earlier and often more useful: when did
this service start behaving strangely? A bounded incident example should let
the operator inspect a sufficiently wide journal window next to the current
unit, process, and cgroup snapshot, looking for the first visible errors,
timeouts, restart bursts, or dependency failures. yesh should make that
evidence easy to collect and compare; it should not invent an anomaly score or
claim that one log line proves the cause.

### Extend finite behaviour measurement

The useful result is a bounded statement about an interval, not a single
snapshot: “during this period this unit or process behaved this way”. The
measurement must have an explicit start, finite duration, and sampling
interval, then compare several snapshots and produce initial/final/maximum
values, counter deltas, resource-pressure changes, process churn, restarts, and
OOM events. The raw points may remain available for verification, but the main
output should summarize direction and observed evidence.

The next extension should use existing read-only sources: cgroup v2
`memory.current`, selected `memory.stat`, `memory.events`, `cpu.stat`,
`pids.*`, `memory.pressure`, `cpu.pressure`, `io.stat`, and the corresponding
host PSI files where readable; `/proc` process records must use PID plus
`start_time`. Gauges and cumulative counters must be compared differently.
Unit identity is the systemd unit plus the observed cgroup path. A unit stop or
restart starts a separate segment; a disappearing process makes the result
incomplete rather than zero.

This is a finite command, not a daemon, alert engine, or `top`-like monitor.
A single invocation can measure an interval in memory. Comparing with a past
invocation requires an explicit, bounded state store of saved JSON snapshots;
kernel cgroup files do not provide historical samples. That state must never
appear as hidden automatic history: its path, schema, retention, permissions,
and unit/cgroup identity must be explicit, with atomic writes and a clear
incomplete result when samples are missing or belong to a different segment.
The result must separate measured change from causal explanation and include
elapsed time and sampling completeness.

Before a persistent `observe` operation is implemented, retention must be
explicit: a maximum age and/or record count, pruning only yesh-owned complete
records, with the chosen policy visible in `--plan` and the report. Without
that policy, periodic observation is a bounded test setup, not unattended
history.

Retention should be a sliding window rather than a fixed lifetime for the
whole feature: a `since 24h` report must keep at least the last 24 hours of
records, plus a small safety margin. A record-count limit is a separate safety
ceiling, not a reason to silently discard data inside the requested window;
the report must say when the available history is incomplete. Longer windows
can later use bounded rollups instead of one file per sample.

Inode usage is a separate storage boundary: one journal per unit should replace
many per-sample files. Retention must account for journal byte size and the
number of unit journals; the temporary file used for an atomic rewrite is an
intentional short-lived extra inode.

The minimum practical history window is 24 hours: the history must support
`summarize ... since 24h` with a small safety margin. Retention must not remove
records inside that requested window.

Prefer one append-only JSON Lines history file per observed unit or
profile instead of one JSON file per sample. Each line is a complete JSON
record, so readers can stream the history and a crash can be limited to the
last incomplete line. Appends still require the existing lock and bounded
record size; retention must rewrite through a temporary file and atomic rename,
never truncate the live history in place. Do not use a JSON array as an append
log: an interrupted final write can leave the whole document malformed.

Reuse the state rules in `docs/underthehood/long-running-and-state.md`: each resource
state record is a versioned JSON document. The current prototype appends
completed records to one bounded JSON Lines file per state directory; separate
JSON files remain a transition format accepted while reading. The record should contain the
systemd manager profile and
unit, the exact cgroup path, a lifecycle identity such as the unit invocation
ID when available, observation times, raw bounded samples, derived deltas, and
an explicit completeness result. It must contain no commands, secrets, or
instructions reconstructed from stored strings. A missing, corrupt, stale, or
identity-mismatched record is a diagnostic result or an error according to the
operation, never an invitation to guess.

For extended model coverage, use one disposable systemd
fixture and three snapshots: stable control, intentional memory growth, and a
resource-pressure case. The positive and negative checks must verify deltas,
counter handling, process identity across PID reuse, restart segmentation,
bounded completion, persisted JSON round-trips, missing-state handling, and
that `--plan` performs no sampling or state writes.

The implemented first slice has this syntax:

```yesh
state = "/tmp/yesh-api-state"

measure service nginx.service for 10m every 10s:
    save to $state
    keep history for 24h
summarize service nginx.service from $state since 2h
```

The first statement performs one finite measurement and writes one completed
record. The second reads bounded records and produces a summary; it does not
start a collector. A scheduler such as a systemd timer may invoke the first
statement repeatedly, but yesh itself remains a foreground finite command.
The first slice and its negative journal/identity checks have been exercised on
the Debian systemd/cgroup-v2 fixture. Broader full-Linux integration coverage
remains separate work.

An additional live check was run on Debian 12 with kernel 6.1 and systemd 252:
`nginx.service` exposed memory, process, RSS, PSI, and OOM counters. The host
did not enable the `io` controller for `system.slice`, so yesh correctly reported
`io.stat.available=unavailable` and an incomplete observation; `cpu.stat.local`
was also absent and was not reported as zero. Parsed per-device `io.stat` values
are now retained in a snapshot when the file is available. `summarize` reports
`rbytes`, `wbytes`, `rios`, `wios`, `dbytes`, and `dios` deltas per device,
keeping device identity explicit and treating a changed device set, an
unavailable point, or a counter reset as incomplete.

The positive I/O fixture was also verified: a temporary service with
`IOAccounting=yes` repeatedly wrote and read a file. On the 6.1 kernel the
snapshot exposed device `8:0`, with approximately 537 MiB read and 663 MiB
written; a 10-second yesh measurement saved 6 complete samples, memory growth
from 70.6 to 133 MiB, and zero OOM kills.

The summary now reports a bounded measured trend for memory and RSS: start,
end, maximum, signed delta, and `rising`/`falling`/`stable`/`unavailable`.
This describes the observed interval and does not claim a cause. The summary
treats `memory.current` and RSS as gauges: it calculates a slope
from all usable timestamped samples (linear regression is the reference model)
and adds `memory_slope_bytes_per_sec`/`rss_slope_bytes_per_sec`. A period with
large movement in both directions is `volatile`, not `stable`.
The summary also reports the first and last observed increase and the peak
timestamp/value when those events are present; these are observations, not
automated incident boundaries.

Resume is an optional, deferred design problem and must not be hidden inside
`measure`. It should return only if a real long-running measurement makes
repeating a finite run impractical.
For overnight or incident history, the primary design is a systemd timer that
starts a finite yesh service repeatedly; `Persistent=true` may catch up a
missed calendar trigger, but cannot reconstruct samples that were never taken.
Timer accuracy, service overlap, manager profile, and the user/system state
directory must remain explicit in the example.
Legacy completed `*.json` files are a transition format for `summarize`; the
primary history is the `measurements.jsonl` journal. An in-progress bounded
measurement may use one `.active.json` checkpoint, rewritten atomically after
each successful sample. A future explicit `resume service` operation may use
that checkpoint only when manager, unit, cgroup, `InvocationID`, schema,
duration, and interval still match. A missing cgroup or changed invocation
turns the checkpoint into an abandoned/incomplete result; it must never be
combined with a new service segment. The checkpoint format must contain no
commands or secrets, must be bounded, and must be safe under competing writers.
Until that operation exists, yesh reads JSON only for `summarize` and never
silently resumes a measurement.
CPU usage and OOM values remain counters: report bounded deltas, handle
identity changes as segment boundaries, and do not apply gauge-rate logic to
them. Do not hide service-specific thresholds in the implementation.

The measurement semantics must be explicit: take the first snapshot
immediately, schedule later samples from a monotonic clock, and finish at the
finite deadline rather than drifting by the time spent collecting a sample.
Require at least two samples for an interval summary and cap the sample count.
Publish a completed JSON record only after the final sample; interruption or a
failed sample leaves no falsely complete record. If `InvocationID` or cgroup
identity changes, do not combine the samples. One JSON record represents one
systemd invocation and one cgroup segment; save the collected points as an
incomplete record with `identity_changed`, then stop. A new segment begins only
with a new explicit measurement, not through an automatic restart-following
loop.

The default user-owned state root is `~/.local/.yesh`; resource measurements
belong below `~/.local/.yesh/resources/`. An explicit state path remains
available for a different layout. If the user-owned directory is
missing, yesh may create the required directory tree with restrictive mode
`0700`; it must not create or repair a privileged parent. `/var/lib/yesh` is
only a pre-provisioned system path for an administrator-managed service. yesh must
not change ownership or invoke `sudo`.

The Debian harness should use only `systemd-run --user` transient units and a
temporary user-owned state directory. Its minimum cases are: three stable
samples; bounded memory growth; a separate `MemoryMax` OOM fixture; CPU
pressure under a temporary quota; restart with changed `InvocationID`; process
disappearance between samples; unreadable metric; valid/corrupt/mismatched
state; creation of a missing user directory with mode `0700`; competing
writers; and byte-for-byte unchanged state under `--plan`.
Every case needs a finite timeout and cleanup verification.
The first Debian run uses the confirmed `/usr/bin/python3`, systemd 252,
and cgroup v2.

Use an existing `nginx.service` as the first recognizable real-world example
when it is available: compare an idle interval with bounded local HTTP load and
include its master/worker process set, cgroup counters, PSI, restarts, and
journal window. Do not install or reconfigure nginx for an ordinary check and
do not use it as the intentional OOM fixture; use a disposable transient unit
for destructive resource cases.

The first fixture should include a positive cgroup-limit/OOM case and negative
or inconclusive cases: a process disappearing during the snapshot, an
unavailable controller, a service with nested child groups, and host pressure
that cannot be attributed to the selected unit. This requires a disposable
Linux systemd/cgroup-v2 environment; a portable test or a single-owner
`/proc` check is not sufficient evidence.

The available Debian 12 host provides a suitable first environment: Linux
6.1, systemd 252, unified cgroup v2, readable memory counters, and working
user-level transient units. A successful 64 MiB `MemoryMax` control run showed
matching `MemoryCurrent`/`memory.current`, `memory.events` and
`memory.events.local`, `memory.stat`, `pids.*`, `cpu.stat`, and
`cgroup.events populated`. A separate 32 MiB run was killed with systemd
result `oom-kill` and was recorded in the user journal. The host has no Go
toolchain, so it is an execution fixture for a locally built Linux binary, not
the place to run Go tests.

The implemented first slice extends the existing Linux
`service resources` snapshot without adding syntax: preserve the full cgroup
scope, record cgroup type and available controllers, distinguish local and
hierarchical events, parse selected `memory.stat` fields, and report
`populated` and incomplete observations explicitly. Keep systemd result and
journal evidence separate from cgroup counters; neither alone proves the
cause of an OOM event.

The current slice records the selected cgroup path, type, available
controllers, and whether a process disappeared during collection. It also
exposes read-only cumulative CPU, memory, swap, and PID-limit counters needed
for interval deltas. It still does not claim that the cgroup is
application ownership or that an OOM cause has been established. It also
exposes read-only cgroup PSI
windows for memory, CPU, and I/O when the kernel provides them; missing PSI
files remain explicitly unavailable. Host-wide `some` PSI windows from
`/proc/pressure` are included when readable, so service pressure can be
compared with machine-wide pressure. Each resource snapshot now also carries
an `observed_at` timestamp and the systemd `InvocationID` when available, so
later samples can be compared over an explicit elapsed interval.

The permission boundary must be visible in the result: this operation never
invokes `sudo`, changes credentials, sends signals, or writes cgroup limits.
It must distinguish `unavailable` data caused by permissions, an absent
controller, or an unsupported file from a real zero; a process that disappears
during collection is an incomplete observation, not a zero-process result.
The human-readable summary must include both an observation state and a reason;
bare `unavailable` values are insufficient for an operator report. The initial
reason set is `unit_not_found`, `no_cgroup`, `metric_unavailable`, and
`process_disappeared`; finer permission-versus-controller causes require
preserving read errors in the metric model.
The report should retain a short operator-facing explanation next to the
machine-readable reason code; `unavailable` must never stand alone in the
human-facing output.

### Next milestone: attribute cgroup growth to processes

The next practical step is not another metric or a new language construct, but
an explanation of the source of already observed growth. Each usable snapshot
should retain a bounded process list for the observed cgroup: PID, Linux
`start_time`, name, and available RSS. Per-process CPU is outside the first
milestone; the service-level `cgroup.cpu.stat` counter in microseconds is the
validated CPU source for now. A PID is not a process identity without
`start_time`: after a process exits, its number may be reused. A process that
disappears while it is being read must make the observation incomplete.

`summarize` should be able to show which processes grew the most, appeared, or
disappeared during the period, and which process was present at the peak. The
report must separate the RSS growth attributed to individual processes from the
total cgroup growth. The difference may be kernel memory, file-backed pages,
swap, or incomplete process reads; it must not be assigned to a process without
evidence.

JSON is for saved measurement state and history: it must be bounded, versioned,
and suitable for yesh to read again. `summarize` is primarily for an operator
and must produce a human-oriented report, not a line that has to be parsed as a
`key=value` table. It needs a short opening sentence containing the period,
unit, growth direction, leading process, and observation completeness; details
may follow on separate lines. The JSON state must keep time, unit, invocation
segment, cgroup, limits, aggregate metrics, process attribution,
incompleteness, and reasons as separate fields so the report can be rebuilt
reliably on a later read.

Do not mix JSON state with human-readable output: publish the state separately
and atomically, then let `summarize` read it and print an explainable
administrative report. If machine-readable output for the report itself is
needed later, make that a separate explicit decision rather than a side effect
of the current output lines.

Keep the human report in a stable order:

```text
Service api.service was observed for 1h 20m; memory was rising, observation complete.

Memory: 44 MiB -> 132 MiB, peak 136 MiB at 03:14:22; MemoryMax: 256 MiB.
Leading RSS growth: worker (+82 MiB); 2 processes appeared and 1 exited during the period.
Events: no OOM observed; no service memory pressure observed.
Scope: data belongs to cgroup ... and InvocationID ... .
Completeness: 8 of 8 samples; this is observed evidence, not a proven cause.
```

The first line must make the situation understandable without reading the full
report. Following lines separate cgroup and process levels, events, and limits.
When the observation is incomplete, state exactly what is missing and which
conclusion is therefore unavailable. Do not use technical values such as
`rising` or `unavailable` in the main prose; retain machine codes in JSON and,
if needed, in a short diagnostic appendix.

Separate units by purpose. Store exact integer byte values in JSON state and
name such fields with a `_bytes` suffix. Format memory in the human report
using binary `KiB`, `MiB`, and `GiB`; do not use ambiguous `KB`. This follows
Linux practice: `ps` documents RSS in kibibytes, while `/proc` retains the
historical `kB` label for the same binary scale. Store `memory.current` and
cgroup limits in bytes. Never present the sum of process RSS as
`memory.current`: they describe different accounting scopes, and RSS itself is
an approximate value.

Store CPU counters in the source unit with that unit explicit. For
`/proc/<pid>/stat`, the source unit is clock ticks; if the human report
converts them to seconds, preserve the clock-tick frequency with the state so
older records can be interpreted correctly. Until a reliable source for that
frequency exists, do not assume the common value `100`, invoke external
`getconf`, or print seconds with false precision: keep the source ticks in the
report and label the unit explicitly.

The same bounded report should include available systemd resource-control
context: `MemoryAccounting`, `MemoryHigh`, `MemoryMax`, `MemorySwapMax`,
`OOMPolicy`, and `ManagedOOM*` settings when exposed by the current manager.
This is read-only configuration and observation: yesh does not change limits,
start `systemd-oomd`, or infer an OOM cause from one field. Properties absent
from the installed systemd version must remain explicitly unavailable rather
than being replaced with an assumed default.

The minimum fixture for this milestone is one unit with several child
processes, separate RSS growth, a process disappearing during collection, PID
reuse, and a restart with a new `InvocationID`. Verify the JSON size bound,
history rereading, incomplete samples, `--plan`, and the 128 MiB ceiling for
the test process. Until these scenarios pass, do not add automatic anomaly
detection, hidden thresholds for suspicious growth, or automatic process
actions.

Process and resource diagnostics are not a monitoring subsystem or a small
Prometheus replacement. The intended scope is a finite snapshot or finite
multi-sample measurement inside a Linux workflow: process tables,
systemd/cgroup service snapshots, resource budgets, and bounded incident
reports. A systemd timer may provide a lightweight history on one node, but
yesh must not grow its own metrics daemon, exporter, or general time-series
database. Feature detection must cover cgroup v1, cgroup v2, unavailable controllers, delegated subtrees, and nested container-like groups. A cgroup snapshot proves membership and accounting scope; it does not prove application ownership or container identity.

The internal positioning for this capability is local resource history for
small Linux systems: informally, “Prometheus for one node without the
Prometheus stack.” It is not a Prometheus replacement or a metrics platform:
there is no permanent server, scrape endpoint, exporter, or shared time-series
database. The value of yesh is that a single systemd host can quickly retain
bounded JSON states and produce an explainable operator report.

Manually assembling two unit files must not be required for the normal path. Add
an explicit user-facing operation such as:

```yesh
observe service user nginx.service every 5m
```

`observe` must not be a hidden wrapper around a long `measure`. Each timer
activation takes one finite service snapshot and saves it as a separate JSON
state, so `every 5m` fully defines the history frequency and no second hidden
duration is needed. The first snapshot is taken immediately when observation is
installed. Detailed multi-sample investigation remains explicit:
`measure service ... for 30s every 5s`. The short form therefore has no hidden
duration, while the detailed form exposes all its boundaries.

It should prepare ordinary user `service` and `timer` units, show the files and
management commands in `--plan`, and on a normal run create them only below
`~/.config/systemd/user/` and activate them through `systemctl --user`. This is
not a yesh daemon, must not invoke `sudo`, modify system unit files, or silently
overwrite someone else's configuration. Stopping and removing the schedule
need an equally explicit inverse operation. If the user manager is unavailable,
or boot-time operation requires administrator-provided lingering, yesh must
report that as a separate permission or environment limitation.
The timer must not invoke the original script containing `observe`, or it would
reinstall itself recursively. Installation should atomically create a separate,
inspectable collector script below `~/.local/.yesh/observations/` containing
only `measure`; the user must be able to read and remove it with the yesh-owned
unit files.
The persistent default should be a user timer backed by ordinary unit files and
`OnCalendar`; `Persistent=true` may catch up one missed calendar trigger but
cannot reconstruct samples that were never taken. After installing the timer,
`observe` should run the first finite measurement immediately, so the period
starts when the command is issued rather than at the next calendar boundary.
A temporary `systemd-run --user --on-calendar` mode may be offered separately,
but transient units must not be presented as durable overnight history. Do not
add cron as a second scheduler. A repeated `observe` must detect yesh-owned
files, refuse to overwrite unrelated configuration, and report what is already
configured; stopping and removing the schedule must remain separate explicit
actions.

Do not add `top`-style observation, a metrics daemon, history storage, general
signals, `pkill`, `cgroup.freeze`, `cgroup.kill`, direct writes to arbitrary
cgroups, or automatic Docker/containerd ownership claims. Docker and containerd
remain external systems unless two independent practical scenarios establish a
smaller, safer interface.

CI is an integration consumer, not a product direction: yesh may run inside a
CI job, but it is not becoming a CI system with matrices, inventories,
artifacts, or remote-worker orchestration.


## Boundaries

- yesh is not an interactive shell, Bash replacement, sandbox, or backup engine;
- rejecting `bash -c` and similar wrappers is a language rule, not an isolation
  boundary;
- `rsync`, `rsnapshot`, `systemctl`, `mount`, `sed`, `awk`, SSH, and package
  managers remain external tools with their native semantics;
- do not add `cd`, `export`, command substitution, TTY/PTY, or broad shell
  compatibility without two independent practical scenarios;
- do not claim power-loss durability, universal OS support, or success of an
  external command in every environment;
- macOS/Darwin work remains deferred.

## Evidence and documentation

- [Practical Examples](practical-examples.md)
- [example map](example-map.ru.md)
- [execution safety](../SAFETY.md)
- [syntax tutorial](syntax-guide.md)
- [Russian roadmap](TODO.ru.md)
- [archived decisions and evidence](TODO-history.md)
