# systemd and journalctl: Simplification Design

The goal is not to replace `systemctl` or `journalctl`, but to remove repeated
administrative boilerplate and make dangerous operations obvious during review.

The useful boundary is a small typed adapter around systemd, not a second
systemd language. yesh should make common questions short — “is this exact
unit healthy?”, “did my restart finish?”, “show the last bounded logs?” — while
keeping the unit name, manager, timeout, and mutation visible in the script.

## Implemented syntax

```yesh
service status api.service
service restart api.service until active within 30s
service wait api.service until active within 30s
journal api.service since 15m lines 100
service status user api.service
journal user api.service since 15m lines 100
```

The shorter forms `service api`, `service.status api`, `service.restart api`,
and `service.wait api running within 30s` are design sketches, not accepted
syntax. The documented forms above are the current interface.

`service status`, `service restart UNIT until active within DURATION`,
`service wait UNIT until active within DURATION`, and
bounded `journal UNIT since DURATION lines N` are implemented. Restart uses
one finite client budget and then queries the exact unit again; it does not use
`systemctl --wait`, which can wait for a long-lived service to terminate.
Journal output is JSON Lines, capped at four MiB, and rejects diagnostics on
stderr so a partial or permission-warning result is not presented as success.
Cursor/boot filters and follow mode remain design targets; bounded `wait` is
implemented.

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
service status api.service
service restart api.service until active within 30s
service wait api.service until active within 30s
journal api.service since 15m lines 100
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
wait`, restart-with-postcondition, and bounded JSON journal forms. They use a
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
systemctl show --no-pager --property=Id,LoadState,ActiveState,SubState api.service
systemctl restart api.service
journalctl --no-pager --output=json --unit api.service --since -15m --lines 100
```

`service restart` runs the second command and then reads the first snapshot
again to verify its postcondition. The exact argv and manager profile remain
visible in `--plan`; this is an implementation boundary, not a promise that
all future systemd operations will remain subprocess-based.

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
