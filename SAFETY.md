# Execution safety

yesh supports scripts that may require administrative privileges. Ambiguity that
can be detected in advance should stop execution.

> **In short:** yesh makes commands, arguments, order, and failure boundaries
> visible. It is not a sandbox and cannot make an arbitrary external program
> safe. Treat a script run as root like any other privileged program.

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

## Confirmed behavior within the tested scope

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

This is confirmed language and runtime behavior, not isolation. The current
implementation does not protect against an arbitrary dangerous external
program and does not promise that an external command will succeed.

The POSIX process model, signal and pipeline behavior, and the scope of the
test corpus are documented in [POSIX runtime details](POSIX.md). The actual
Linux checks and their limits are collected in the [Linux integration matrix](docs/underthehood/linux-integration.md);
positive and negative operational scenarios are listed in [Practical Examples](docs/practical-examples.md)
and the [current project plan](docs/TODO.md). A result from one fixture does not
mean support for every operating system, filesystem, privilege policy, or
external command.

## Two separate safety layers

The language semantics and the external-command policy are different layers:

- **Language semantics**: no `eval`, implicit shell expansion, uncontrolled
  background execution, or unbounded loops; values and `argv` retain explicit
  boundaries.
- **Execution policy**: which external program is allowed, under which user,
  with which arguments, paths, environment, and privileges.

Rejecting `bash -c` belongs to the first layer. It prevents a script from
bypassing yesh parsing and runtime rules, but it is not an isolation boundary:
a renamed executable, `awk system()`, `find -exec`, or a program that creates a
new session can still perform dangerous work. The second layer requires a
separate, reviewed policy for external programs.

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

## Containers and images

yesh is not a container boundary. `docker`, `podman`, `skopeo`, `cosign`,
`buildah`, and `umoci` remain external programs with their own authority and
trust models.

- Treat a Docker or Podman socket, a rootful runtime, `--privileged`, host
  namespaces, host networking, and host-path mounts as high-impact privileges.
- Prefer an image reference by digest. Check the digest, policy or signature,
  and runtime configuration before starting a temporary container; start the
  same digest that was checked, not a mutable tag.
- Keep the old container, volume, and external entry point until the new
  container passes its finite state and HTTP checks. Removing the old object is
  a separate action after success.
- Call a return to the previous container or unit a compensating rollback, not
  a transaction. An interruption after the external switch can leave the new
  container active.
- Do not put registry credentials, tokens, or private keys in a yesh script or
  an argument that `--plan` or an error may display. Use the runtime's external
  credential mechanism.
- A multi-stage image and a small runtime reduce contents and attack surface;
  they do not prove the base image, dependencies, signature, or application are
  trustworthy. Keep only the external tools required by the scenario.

## Network and exposure checks

- A successful TCP connection proves only that something accepted a connection
  on that address and port. It does not prove that the expected application is
  serving there. Before switching traffic, check the expected DNS records,
  route, SNI/certificate, HTTP `Host`, status, and a bounded control response.
- Treat a published container port as an exposure decision. Check the runtime's
  published address and port against `ss`, interface addresses, and firewall
  rules. Reject an unexpected `0.0.0.0`, IPv6 exposure, administrative port, or
  network attachment.
- Test rootless and rootful runtimes separately. A local check does not prove
  reachability from every external network, and runtime metadata does not prove
  the actual packet path.
- Treat `nft -c` or an equivalent syntax check as necessary but insufficient.
  Applying a firewall ruleset can terminate the only management path; test
  changes on an isolated node or with console access and retain a reviewed
  recovery path.
- DNS, TLS, HTTP, and firewall checks are observations around an external
  operation. They do not make yesh a network monitor, certificate authority,
  firewall policy engine, or transaction manager.

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
- Keep secrets in the SSH agent, protected configuration, or an external secret
  store, never in a yesh script.
- Do not put secrets in a script that will be planned or logged: `--plan` shows
  expanded arguments, and command errors can expose bounded `stderr`.
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

A future execution-policy check should have three explicit outcomes:

| Result | Meaning |
| --- | --- |
| `allowed` | The operation matches the reviewed policy; this does not mean that it will succeed. |
| `denied` | A concrete policy violation was found; the operation must not start. |
| `unknown` | There is not enough information to decide; it must not be silently treated as `allowed`. |

For an unknown external command, yesh must explicitly state that its privilege
requirements are unknown. Successful preflight does not prove future access:
permissions, ACLs, capabilities, set-user-ID behavior, remote policy, and paths
may change between checking and acting. A privileged script must have an
explicitly reviewed command, account, environment, and operating-system policy.

## Requirements for v1.0.0

The following are required project work, not optional advice:

- define controlled `PATH` and environment policies for privileged execution;
- keep language restrictions separate from the external-command policy, with
  explicit `allowed`/`denied`/`unknown` results and no implication that
  `allowed` guarantees execution success;
- implement argument validation or typed wrappers for high-impact external
  commands;
- maintain fuzz checks for lexing, parsing, value expansion, redirects, and
  the runtime using randomly generated data;
- add `govulncheck` and dependency scanning to CI;
- publish a threat model and release security checklist;
- maintain an opt-in privileged loopback/storage fixture in a disposable Linux
  VM or on a separate Linux host; its results must remain clearly scoped to
  that fixture and environment;
- either document and test isolation of untrusted scripts, or clearly state
  that yesh does not provide such an isolation mechanism.
