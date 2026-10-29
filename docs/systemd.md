# systemd and journalctl: Simplification Design

The goal is not to replace `systemctl` or `journalctl`, but to remove repeated
administrative boilerplate and make dangerous operations obvious during review.

## Target syntax

```yesh
service api
service.status api
service.restart api
service.wait api running within 30s
journal api since 15m lines 100
```

Initially each operation should lower to ordinary commands:

```text
systemctl is-active api
systemctl restart api
systemctl is-active --quiet api
journalctl -u api --since 15m -n 100 --no-pager
```

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

## Open design question

Should `service.*` and `journal` be typed built-in operations or safe macros over
`systemctl` and `journalctl`? The preferred MVP direction is an internal
command with the same transparent argv plan.
