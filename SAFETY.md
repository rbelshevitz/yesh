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

## High-impact Linux operations

The following operations need an explicit review of the expanded arguments,
the resolved paths, the current mount, and the account that will run them.
This is a risk catalogue, not a complete detector: an arbitrary external
program can hide the same effect behind different names or configuration.

- `rm -r`, `rm --no-preserve-root`, `find -delete`, and similar recursive
  removal can destroy a tree. A missing or wrong path is still dangerous;
  `rm`'s own safeguards are not a yesh policy. See the [GNU `rm(1)` manual](https://www.man7.org/linux/man-pages/man1/rm.1.html).
- `mkfs.*`, `wipefs`, `dd` to a device, and block-device writes can destroy a
  filesystem or its metadata. Verify the device identity and require a
  disposable fixture before testing. See the [`mkfs(8)` manual](https://man7.org/linux/man-pages/man8/mkfs.8.html).
- `mount --bind`/`--rbind`, `mount --move`, remounts, and propagation changes
  alter which files are visible and where writes go. A read-only bind does not
  necessarily make the underlying filesystem read-only. See the [`mount(8)` manual](https://www.man7.org/linux/man-pages/man8/mount.8.html).
- `systemctl stop`, `restart`, `kill`, `clean`, `isolate`, and operations that
  reload or replace service configuration can interrupt unrelated work. The
  systemd manual explicitly calls `isolate` dangerous because it can stop
  processes outside the requested target. See the [systemd `systemctl` manual](https://www.freedesktop.org/software/systemd/man/systemctl.html).
- `rsync --delete*`, `--delete-excluded`, `--delete-missing-args`, and
  `--remove-source-files` can remove destination or source entries.
  `--inplace` can leave an existing destination file partially updated after
  interruption; `--link-dest` creates hard-link relationships between trees;
  `--partial`/`--partial-dir` leave or reuse transfer state and must not use a
  directory writable by other users. `--delay-updates` changes when updated
  files become visible. See the [official rsync manual](https://www.man7.org/linux/man-pages/man1/rsync.1.html).

For these operations, yesh can make the command and failure boundary visible,
but it cannot prove that a path, device, remote host, service unit, or external
program means what the operator intended. A future command-specific policy
should classify known high-impact arguments before start; it must not pretend
that a blacklist is a sandbox.

An `rsnapshot` interval is also a high-impact operation: it can rotate or
remove older levels according to `retain` and create hard links between
snapshots. Before applying one, inspect the effective configuration, snapshot
root, mount identity, `lockfile`, `no_create_root`, `configtest`, and test mode
`-t`. An approval file does not replace those checks.

## Privilege and `sudo`

yesh must never add `sudo` implicitly. Whether a command needs extra privilege
depends on the operation, ownership, ACLs, capabilities, mount options, service
policy, and (for SSH/rsync) the remote account. `sudo` itself only runs a
command as another user when the local security policy permits it; it is not a
proof that the command is safe. See the [`sudo(8)` manual](https://www.man7.org/linux/man-pages/man8/sudo.8.html).

The useful behavior is therefore:

- classify known operations as read-only, user-writable, privileged, or
  environment-dependent;
- check the current identity and the relevant local facts when that check is
  reliable and read-only;
- stop before a state-changing operation when a required privilege is absent;
- show which stage requires privilege and which account will execute it;
- never repair the situation by wrapping an arbitrary command in `sudo`.

For an unknown external command, yesh must report that privilege requirements
are unknown. A successful preflight cannot prove future access: permissions,
ACLs, capabilities, set-user-ID programs, remote policy, and paths can change
between the check and the operation. Privileged workflows should use an
explicitly reviewed command, account, environment, and operating-system policy.

## Mandatory security work before a stable release

The following are required project work, not optional recommendations:

- define controlled `PATH` and environment policy for privileged/runbook
  execution;
- implement argument validation or typed wrappers for high-impact external
  commands;
- maintain lexer, parser, value-expansion, redirect, and runtime fuzzing;
- add `govulncheck` and dependency scanning in CI;
- publish a threat model and release security checklist;
- keep a privileged loopback/storage fixture in a disposable VM or Rocky host;
- document and test full OS-level isolation for untrusted scripts, or clearly
  state that yesh does not provide it.
