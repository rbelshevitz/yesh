# systemd and journalctl: Simplification Design

The goal is not to replace `systemctl` or `journalctl`, but to remove repeated
administrative boilerplate and make dangerous operations obvious during review.

The useful boundary is a small typed adapter around systemd, not a second
systemd language. yesh should make common questions short — “is this exact
unit healthy?”, “did my restart finish?”, “show the last bounded logs?” — while
keeping the unit name, manager, timeout, and mutation visible in the script.

## Implemented syntax

```yesh
service status nginx.service
service restart nginx.service until active within 30s
service wait nginx.service until active within 30s
journal nginx.service since 15m lines 100
service resources nginx.service
service status user api.service
journal user api.service since 15m lines 100
```

The shorter forms `service api`, `service.status api`, `service.restart api`,
and `service.wait api running within 30s` are design sketches, not accepted
syntax. The documented forms above are the current interface.

Here `service` is the operation keyword; there is no separate `unit` keyword.
The systemd unit is the following positional value, as in
`service resources nginx.service`. In tutorial examples, `unit` may also be an
ordinary variable name such as `unit = nginx.service`; it is not special service
syntax.

`service status`, `service restart UNIT until active within DURATION`,
`service wait UNIT until active within DURATION`, and
bounded `journal UNIT since DURATION lines N` are implemented. Restart uses
one finite client budget, then polls the exact unit until the required
postcondition or the deadline; it does not use `systemctl --wait`, which can
wait for a long-lived service to terminate. Journal output is JSON Lines,
capped at four MiB, and rejects diagnostics on stderr so a partial or
permission-warning result is not presented as success. Cursor/boot filters and
follow mode remain design targets; bounded `wait` is implemented.

`service resources UNIT` is a read-only snapshot of a Linux unit and its cgroup:
`ControlGroup`, the unit `InvocationID` when available, the main PID, cgroup type and available controllers, process
count, total RSS, selected cgroup v2 memory/task/CPU counters, `memory.stat`
values, memory/CPU/I/O pressure windows where available, local and hierarchical
OOM events, `populated`/`frozen` state, and a process table. The main PID is not
treated as the whole service: child
processes come from the unit cgroup and its subgroups. `processes_incomplete=yes`
means that a process disappeared while the finite snapshot was being collected;
it is not silently treated as a complete process list.
The report also includes host-wide PSI windows where `/proc/pressure` is
readable, so service pressure can be compared with pressure on the machine.
The operation never invokes `sudo`, changes credentials, sends signals, or
writes limits. It reads only what the current user and systemd manager expose:
unavailable fields are printed as `unavailable`; the report also includes an
`observation` state and `observation_reason` such as `unit_not_found`,
`no_cgroup`, `metric_unavailable`, or `process_disappeared`. A process that disappears
during the snapshot is marked as an incomplete observation. Permission denied,
an absent controller, and an unsupported file are not reported as zero or as
success. The operation requires Linux, an available systemd manager, and cgroup
v2; it is a point-in-time observation, not a guarantee of future service
behavior.
The preceding `RESOURCE_OBSERVATION` line is a plain operator-facing sentence;
it explicitly says when unavailable values are not zero.

The first finite measurement and summary forms are:

```yesh
# This directory contains measurements.jsonl and its lock file; summarize
# reads the saved history from there. Use a persistent user-owned directory
# for regular operation; /tmp is only for this exercise.
state_dir = "/tmp/yesh-nginx-resources"

service status nginx.service
measure service nginx.service for 30s every 5s:
    save to $state_dir
    keep history for 24h
summarize service nginx.service from $state_dir since 1h
journal nginx.service since 15m lines 40
```

This is a read-only incident check for an existing nginx service: yesh measures
the master and worker processes already assigned to nginx, then places the
resource trend beside a bounded journal window. It does not start nginx or
generate HTTP traffic; use a separate, operator-approved load source if the
comparison needs traffic.

If `save to` is omitted, yesh uses the user-owned directory
`~/.local/.yesh/resources/<unit>/`; for this example that is
`~/.local/.yesh/resources/nginx.service/`. yesh creates only the missing
directories below the user's home with mode `0700` (owner access only). It does
not invoke `sudo`, create or repair a privileged parent, or change ownership.
An explicit `save to` path must already be writable by the current user; yesh
does not change its existing permissions.

The useful part is not collecting one more number. In one readable script, yesh
keeps together the service, the observation period, the saved history, the
changes in memory and RSS, and the journal window for the same period. An
administrator can read the check from top to bottom and see what it observes,
where it saves the result, and what counts as an incomplete observation. The
Linux commands remain visible; yesh gives them a clear order and shared rules.
A Bash comparison below shows the commands involved; it is not part of the
yesh scenario.

`measure` takes the first snapshot immediately, samples until its finite
deadline, and appends one completed JSON Lines record only after collection
finishes. `keep history for 24h` removes only records older than that window
after a successful measurement; a failed measurement leaves the old history
unchanged.
Here, `complete=true` means that the scheduled interval finished and the unit
identity remained stable. It does not mean that every optional controller was
available: an absent `io` controller, PSI file, or accounting field remains
`unavailable` in the samples and is called out by the summary.
This is deliberately different from the one-snapshot `observation=partial`
line: that line reports missing fields in that snapshot, while `complete=true`
reports that the finite interval and service identity were valid. For example,
the Debian user-service fixture had unavailable block I/O but still produced a
complete two-second measurement; a missing cgroup or a process disappearing
during collection makes the saved measurement incomplete.
The block requires the retention directive; `save to` may be omitted to use the
default state directory. If no retention policy is needed, use the short
one-line form without a block.
`summarize` reads matching records from the state directory; it does not start
a service, sample cgroups, or repair state. The state path is a directory and
is created below `~/.local/.yesh/resources/` with restrictive permissions when
it does not exist. An explicit `save to $state_dir` / `from $state_dir` path is
available for a different layout. A restart or
cgroup identity change ends the current record as incomplete rather than
combining different service invocations.

The saved JSON records retain exact byte values and derived fields such as
`memory_start`, `memory_end`, `memory_max`, `memory_delta`, and the measured
trend and slope. `summarize` renders those records as an operator report rather
than printing a `key=value` table. It opens with the unit, observed duration,
memory direction, and observation completeness, then describes memory, RSS,
CPU/OOM counters, process RSS growth, process churn, scope, and limitations.
Process attribution compares adjacent complete samples only within one
`InvocationID` and cgroup; a PID reuse is a new process. This is measured
evidence, not a causal diagnosis.

For overnight history, keep scheduling in systemd and keep each yesh run
finite. The measurement script can contain the two statements above; a user
service and timer can invoke it like this:

```ini
# ~/.config/systemd/user/yesh-nginx-resources.service
[Unit]
Description=Bounded yesh resource measurement for nginx.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/yesh /home/ops/yesh-nginx-resources.yesh
```

```ini
# ~/.config/systemd/user/yesh-nginx-resources.timer
[Unit]
Description=Periodic yesh resource measurement for nginx.service

[Timer]
OnCalendar=*:0/5
AccuracySec=1s
Persistent=true
Unit=yesh-nginx-resources.service

[Install]
WantedBy=timers.target
```

The measurement duration must be shorter than the timer interval so runs do
not overlap. `Persistent=true` can trigger a missed calendar run after the
timer was inactive; it cannot reconstruct samples that were never collected.
The timer records scheduling history, while yesh JSON records resource
samples. A system-manager installation needs an explicit system-owned state
path instead of the user default.

`DURATION` currently uses the Go duration form accepted by yesh, such as
`300ms`, `2s`, `15m`, or `1h30m`; it is intentionally narrower than the full
`systemd.time` syntax.

An optional manager profile, `system` or `user`, may precede the unit. Omitting
it preserves the older syntax and selects the system manager. The `user`
profile adds `--user` to both `systemctl` and `journalctl`; it does not imply
that a persistent user journal exists or that the caller has read permission.

`service wait` does not start or restart anything. It checks immediately and
polls until the finite deadline, reporting the last `ActiveState` and
`SubState` on timeout. This is manager state, not application readiness. For
`Type=oneshot`, an `active` postcondition is meaningful only with
`RemainAfterExit=yes`; without it a successful oneshot commonly becomes
`inactive/dead`, which yesh does not reinterpret.
The `failed`, `not-found`, and `masked` states are reported immediately;
transitional `inactive`, `activating`, and `deactivating` states are polled
because an external actor may change them.

## What the simplification must remove

The adapter should remove only repetition and unsafe defaults:

| Repeated systemd work | yesh improvement |
| --- | --- |
| `systemctl show` plus a hand-written property list | one bounded service snapshot with named fields |
| start/restart followed by a polling loop | one action with a required postcondition and timeout |
| `systemctl status` output parsed by text tools | structured state from an allowlist of properties |
| `journalctl` with pager and unbounded output | bounded records, explicit time range, no pager |
| different handling of system and user managers | an explicit manager choice in the operation |

The adapter must not hide `sudo`, unit selection, mutations, or failures. It
must not accept globs or silently operate on several units. `daemon-reload`,
`reload`, `restart`, `enable`, `disable`, `mask`, and `unmask` are different
operations and must remain different in yesh.

## Important systemd traps

These are part of the design boundary, not implementation trivia:

- An alias is a valid explicit systemd target, but `systemctl show` may report
  the canonical `Id`. The current MVP rejects that mismatch explicitly and
  says that aliases are unsupported; accepting aliases requires a separate
  canonical-identity policy.
- `foo@.service` is a template, while `foo@blue.service` is an instance. The
  first is rejected as a target; concrete instances remain explicit names.
- `Type=oneshot` without `RemainAfterExit=yes` transitions to inactive/dead
  after successful completion. `active` is therefore not a universal meaning
  of “the operation succeeded”.
- `systemctl restart` waits for the manager job unless `--no-block` is used,
  but completion of the manager job is not application readiness. A later
  check may need `Result`, process exit status, a readiness probe, or a
  service-specific postcondition.
- `--system` and `--user` select different managers. Journal access is also
  permission-dependent, and `--user` has restrictions when persistent user
  logging is disabled.
- `journalctl --unit=foo.slice` includes child-unit records. A typed journal
  query must not describe that as a single-service filter.
- Empty journal output is a valid result. It must remain distinct from denied
  access, an unavailable manager, rotation beyond the requested window, or an
  invalid/partial machine-readable response.

## Minimal useful interface

The first useful layer is deliberately small:

```yesh
service status nginx.service
service restart nginx.service until active within 30s
service wait nginx.service until active within 30s
journal nginx.service since 15m lines 100
```

The result of `service status` should distinguish at least `active`, `inactive`,
`failed`, `activating`, `deactivating`, `masked`, `not-found`, unavailable
manager, and permission denied. `service restart` must report both the command
result and the verified final state. A successful `systemctl` client return is
not by itself proof that the service reached the requested state.

The exact grammar remains subject to parser review. The semantic rule is more
important: every mutating service operation has an explicit postcondition or
returns only the manager's result; it never pretends that a timeout proves
stopping.

## Current boundary and next work

The implemented MVP contains read-only `service status`, bounded `service
wait`, restart-with-postcondition, bounded JSON journal, and read-only
`service resources` forms. They use a
fixed `systemctl show` allowlist, exact unit identity, explicit system/user
profiles, finite deadlines, and transparent `--plan` argv.

Still separate work: start/stop/reload, `oneshot` completion semantics,
queued-job and readiness semantics after restart, cursor/boot and field allowlists,
journal rotation/cursor policy, follow/streaming, and a transient-service
backend based on `systemd-run`. None of that is implied by the current MVP.

These operations should be typed runtime commands with the same transparent
`argv` plan as external commands, rather than textual macros. That keeps
`--plan`, diagnostics, and future native D-Bus support on one path.

The current subprocess boundary is ordinary, inspectable commands:

```text
systemctl show --no-pager --property=Id,LoadState,ActiveState,SubState nginx.service
systemctl restart nginx.service
journalctl --no-pager --output=json --unit nginx.service --since -15m --lines 100
```

`service restart` runs the second command and polls the restart snapshot until
the postcondition or the finite deadline. The exact argv and manager profile
remain visible in `--plan`; this is an implementation boundary, not a promise
that all future systemd operations will remain subprocess-based.

## Safety rules

- unit names are one value, with no hidden shell parsing;
- `restart`, `stop`, `disable`, and `mask` are explicit mutating operations;
- `--plan` shows the command and unit but changes no state;
- `wait` has a required timeout and a finite number of checks;
- `journal` bounds time and line count and disables the pager;
- yesh never adds `sudo` automatically;
- diagnostics preserve exit status, signal, timeout, and stderr;
- unbounded journal streaming requires an explicit `follow` mode;
- `follow` is outside the MVP until cancellation and TTY closure are modeled.

## Masked units are a state, not a parser error

`LoadState=masked` is a valid and important observation. A status/preflight
operation must report it rather than trying to repair it. A requested
`start`, `restart`, or `ensure active` should fail with an actionable message:
the unit is masked and requires an explicit operator decision. yesh must never
implicitly run `systemctl unmask` as part of start/restart.

If unmasking is eventually supported, it must be a separate explicit mutation:

```text
systemctl unmask nginx.service
systemctl start nginx.service
```

The plan must show both operations, and the postcondition must verify the unit
state. A masked unit can still be inspected, logged, and included in a
read-only fleet report.

## Example and fixture map

| Concern | Example | Fixture evidence |
| --- | --- | --- |
| exact unit and fixed fields | `examples/service-status.yesh` | allowlist, canonical-id check, no mutation |
| restart is not readiness | `examples/service-restart.yesh` (fixture-backed) | inactive postcondition stops the script |
| bounded polling | `examples/service-wait.yesh` | transition to active, timeout, failed-state fast error |
| bounded journal output | `examples/service-journal.yesh` | JSON validation, stderr failure, empty output success |
| unit resource snapshot | `examples/service-resources.yesh` | cgroup/process correlation and bounded resource fields |
| bounded service-memory diagnosis | `examples/service-memory-diagnosis.yesh` | cgroup memory/OOM counters, processes, and journal context |

For an opt-in check against a real systemd manager, use
[`scripts/yesh-fixture-test.sh`](../scripts/yesh-fixture-test.sh). It creates
unique disposable units only in `/run/systemd/system`, tests real `status`,
`restart`, and `wait`, demonstrates `oneshot` without `RemainAfterExit` and a
timeout while a unit is `activating`, then removes only its own files through a
`trap`. It requires interactive `sudo` and must not be run against production
services.

The examples are intentionally comments-heavy. Each has a nearby Bash
comparison and the practical fixture runs the actual file with fake
`systemctl`/`journalctl`, checking argv, output, ordering, failure propagation,
and `--plan` without touching a real manager.

## Design decision

`service` and `journal` are typed built-in operations, not text macros over
`systemctl` and `journalctl`. Their transport may later move to native D-Bus,
but identity, states, deadlines, diagnostics and postconditions must remain
observable and equivalent.
