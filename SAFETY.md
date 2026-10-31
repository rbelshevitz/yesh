# Execution safety

yesh supports scripts that may require administrative privileges. Ambiguous or
unsafe situations should fail before a process starts whenever possible.

## What ordinary mode forbids

```yesh
while true:              # no: no demonstrable end
    echo loop

command &                # no: uncontrolled background process
command |                # no: pipeline without a consumer
| command                # no: pipeline without a producer
bash -c text             # no: bypasses yesh parsing and runtime
eval text                # no: text does not become code
```

## Runtime limits

- `for` accepts only finite lists;
- the number of iterations is bounded;
- an external command has a time limit;
- a pipeline timeout cancels the whole process group and then applies
  SIGTERM → SIGKILL;
- a stuck child process must not survive termination of the job's process
  group;
- a pipeline has at least two stages and is connected to standard output or a
  redirection;
- the runtime starts and waits for every pipeline stage;
- a non-zero exit code is not lost;
- an error identifies the file, line, command, and bounded `stderr`; this does
  not mean that `stderr` is automatically scrubbed of secrets.

Where possible, limits must be checked before execution. The runtime must not
start a process that later becomes unreachable because a pipeline reader is
missing.

The source of a `.yesh` script is limited to 4 MiB before parsing. Additional
limits apply to loops, process snapshots, regular expressions, and Linux
storage-query output.

## Current guarantees

- commands are started as argument vectors, without reparsing a command string
  through a shell;
- variable values are not reparsed as operators, and lists expand only
  explicitly;
- `while`, background execution, command substitution, `eval`, and known shell
  wrappers or nested shell stages are rejected;
- pipelines are checked before execution, and a failure stops later work;
- a pipeline timeout terminates its owned process group and waits for child
  processes;
- redirects are checked before execution, and unsafe special files or input/
  output aliasing are rejected;
- `--plan` displays expanded arguments without starting commands or opening
  redirect files.

These are language and runtime guarantees, not isolation. The current
prototype does not protect against an arbitrary dangerous external program.

## Residual risks

An external program can modify the system, start another shell internally,
follow a path changed after preflight, or interpret a syntactically valid
argument dangerously. Direct argument execution prevents shell reparsing, but
does not validate the meaning or authority of every program.

Name checks for shells, interpreters, and wrappers are a narrow execution
policy, not an isolation boundary. A renamed program, `awk system()`,
`find -exec`, a script written to disk, or a program that creates a new session
can bypass such checks.

Do not run untrusted scripts as root. For isolation, use a dedicated account,
controlled `PATH` and environment, filesystem permissions, and operating-system
controls such as namespaces, cgroups, seccomp, MAC policy, and executable
allowlists.

## Operational rules

- Treat an external command name and its arguments as policy, not merely as
  syntax.
- Put `--` before path operands when the external program supports it.
- Validate destinations, identities, units, hosts, and numeric limits before
  invoking `rsync`, `ssh`, `mount`, or `systemctl`.
- Treat deletion flags as a separate approval boundary. For `rsync`, require a
  plan review, an explicit destination and mount point, a `--max-delete` limit,
  and a policy for excluded files and missing sources.
- Treat an rsnapshot configuration as executable policy and inspect its
  effective `rsync_long_args`, `sync_first`, `snapshot_root`, `no_create_root`,
  `lockfile`, and backup-script entries.
- Keep secrets in the SSH agent, configuration, or an external secret store,
  never in a yesh script.
- Use `--plan` for review, remembering that a plan is not a transaction and
  does not eliminate races between checking and acting.

## High-impact Linux operations

The following operations require a separate review of expanded arguments, real
paths, the current mount point, and the account that will run them. This is a
risk catalogue, not a complete detector: an external program can hide the same
effect under another name or in its configuration.

- `rm -r`, `rm --no-preserve-root`, `find -delete`, and similar recursive
  removal can destroy a tree. A wrong or replaced path is still dangerous;
  `rm` safeguards do not replace yesh policy. See the [GNU `rm(1)` manual](https://www.man7.org/linux/man-pages/man1/rm.1.html).
- `mkfs.*`, `wipefs`, `dd` to a device, and block-device writes can destroy a
  filesystem or its metadata. Verify the device first and test only on a
  disposable fixture. See the [`mkfs(8)` manual](https://man7.org/linux/man-pages/man8/mkfs.8.html).
- `mount --bind`/`--rbind`, `mount --move`, remounts, and propagation changes
  alter file visibility and where writes go. A read-only bind mount does not
  necessarily make the source filesystem read-only. See the [`mount(8)` manual](https://www.man7.org/linux/man-pages/man8/mount.8.html).
- `systemctl stop`, `restart`, `kill`, `clean`, `isolate`, and reloading or
  replacing service configuration can interrupt unrelated work. The systemd
  `systemctl` manual explicitly calls `isolate` dangerous because it can stop
  processes outside the selected target. See the [systemd `systemctl` manual](https://www.freedesktop.org/software/systemd/man/systemctl.html).
- `rsync --delete*`, `--delete-excluded`, `--delete-missing-args`, and
  `--remove-source-files` can remove destination or source entries.
  `--inplace` can leave an existing destination file partially changed after
  interruption; `--link-dest` creates hard links between trees;
  `--partial`/`--partial-dir` leave or reuse transfer state, so their directory
  must not be writable by other users. `--delay-updates` changes when updated
  files become visible. See the [official rsync manual](https://www.man7.org/linux/man-pages/man1/rsync.1.html).

For these operations, yesh can make the command and failure boundary visible,
but cannot prove that a path, device, remote host, service, or external program
means what the operator intended. The next step is a separate policy for known
commands and dangerous arguments before execution; a blacklist must not be
presented as isolation.

An rsnapshot interval is also a high-impact operation: `retain` can move or
remove older levels and create hard links between snapshots. Before applying
one, inspect the effective configuration, snapshot root, mount point,
`lockfile`, `no_create_root`, `configtest`, and test mode `-t`. An approval file
does not replace these checks.

## Privilege and `sudo`

yesh must never add `sudo` implicitly. Whether extra privilege is needed depends
on the operation, owner, ACLs, process capabilities, mount options, service
policy, and, for SSH/rsync, the remote account. `sudo` only runs a command as
another user when the local policy permits it; it does not prove that the
command is safe. See the [`sudo(8)` manual](https://www.man7.org/linux/man-pages/man8/sudo.8.html).

The appropriate yesh behavior is therefore to:

- classify known operations as read-only, writable in user-owned locations,
  privilege-requiring, or environment-dependent;
- check the current account and relevant facts when the check itself is safe
  and read-only;
- stop before changing state when required privilege is absent;
- show which stage requires privilege and which account will run it;
- never fix the situation by wrapping an arbitrary command in `sudo`.

For an unknown external command, yesh must explicitly state that its privilege
requirements are unknown. Successful preflight does not prove future access:
permissions, ACLs, capabilities, set-user-ID behavior, remote policy, and paths
may change between checking and acting. A privileged script must have an
explicitly reviewed command, account, environment, and operating-system policy.

## Mandatory work before a stable release

The following are required project work, not optional advice:

- define controlled `PATH` and environment policies for privileged execution;
- implement argument validation or typed wrappers for high-impact external
  commands;
- maintain fuzz checks for lexing, parsing, value expansion, redirects, and
  the runtime using randomly generated data;
- add `govulncheck` and dependency scanning to CI;
- publish a threat model and release security checklist;
- keep a privileged loopback/storage fixture in a disposable VM or on a
  separate Rocky Linux host;
- either document and test isolation of untrusted scripts, or clearly state
  that yesh does not provide such an isolation mechanism.
