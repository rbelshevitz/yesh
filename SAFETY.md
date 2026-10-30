# Execution safety

yesh is designed for readable Unix automation, including scripts that may run
with administrative privileges. Ambiguous or unsafe runtime situations should
fail before a child process is started whenever possible.

## Current guarantees

- Commands are executed as an argument vector, without reparsing a constructed
  command string through a shell.
- Variable values are not reparsed as operators, and lists expand only
  explicitly.
- `while`, implicit background execution, command substitution, `eval`, and
  nested shell stages are rejected.
- Pipelines are preflighted before execution; failures stop later work.
- Pipeline timeouts terminate the owned process group and reap its children.
- Redirect targets are checked before execution; unsafe special files and
  read/truncate aliasing are rejected.
- `--plan` displays expanded argv without starting external commands or opening
  redirect files.
- Script source is capped at 4 MiB before parsing; bounded loops, process
  snapshots, regexes, and Linux storage-query output provide additional
  resource limits.

These are language and runtime guarantees, not a sandbox.

## Residual risks

An external command can still modify the system, invoke another interpreter
through its own implementation or shebang, follow a path changed after a
preflight check, or interpret a syntactically valid argument dangerously.
Direct argv execution prevents shell re-parsing; it does not validate the
semantics or authority of every external program.

The name-based rejection of shells, interpreters, and pipeline wrappers is a
narrow execution policy, not an isolation boundary. Renamed programs, `awk
system()`, `find -exec`, scripts written to disk, and programs creating new
sessions can bypass such checks.

Do not run untrusted yesh scripts as root. Use a dedicated account, controlled
`PATH` and environment, filesystem permissions, and operating-system controls
such as namespaces, cgroups, seccomp, MAC policy, and executable allowlists
when isolation is required.

## Operational rules

- Treat external command names and arguments as a policy decision, not merely
  as syntax.
- Put `--` before path operands when the external program supports it.
- Validate destinations, identities, units, hosts, and numeric limits before
  invoking tools such as `rsync`, `ssh`, `mount`, or `systemctl`.
- Treat deletion flags as a separate approval boundary. The current rsync
  examples do not use `--delete`; if a retention/mirror workflow needs it,
  require a `--plan`/`--dry-run` review, an explicit destination and mount
  identity, a bounded `--max-delete`, and a policy for excluded files and
  missing sources. Never infer permission to delete from the presence of a
  source path.
- Treat an rsnapshot configuration as executable policy: inspect its effective
  `rsync_long_args`, `sync_first`, `snapshot_root`, `no_create_root`,
  `lockfile`, and backup-script entries before applying an interval. A clean
  yesh wrapper cannot make a dangerous rsnapshot.conf safe by itself.
- Keep credentials in the user's SSH agent/config or an external secret store;
  never put passwords or private keys in a yesh script.
- Use `--plan` for review, but remember that a plan is not a transaction and
  does not eliminate TOCTOU races.

## Security backlog

- controlled `PATH` and environment policy for privileged/runbook execution;
- argument validation or typed wrappers for high-impact external commands;
- lexer, parser, value-expansion, redirect, and runtime fuzzing;
- `govulncheck` and dependency scanning in CI;
- a documented threat model and release security checklist;
- a privileged loopback/storage fixture in a disposable VM or Rocky host;
- full OS-level isolation for untrusted scripts.
