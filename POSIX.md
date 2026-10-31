# POSIX runtime details

yesh keeps the Unix process model but does not try to copy every POSIX shell
rule. The differences must be deliberate and documented.

This document describes the current POSIX/Linux contract and the boundary of
the evidence. The repository currently contains more than 200 test functions
(214 at the release checkpoint) and four fuzz targets. The sections below say
what they check; they are not a promise for every kernel, distribution, or
external utility.

## Pipelines

Each pipeline stage is a separate process. The runtime creates and connects a
pipe between adjacent stages, closes unused ends in the parent after startup,
waits for every stage, and preserves each stage's result.

If an extra process keeps a write end open, the reader never sees EOF. If the
reader exits, a writer may receive `SIGPIPE`/`EPIPE`; this is a normal Unix
pipeline condition and is not necessarily a yesh error. See [Linux
pipe(7)](https://man7.org/linux/man-pages/man7/pipe.7.html).

POSIX shells commonly return the status of the last pipeline command. yesh
deliberately uses safer behavior: a pipeline succeeds only when every stage
succeeds. This is similar to `pipefail`, but built in and not an optional flag.

## Timeouts and termination

A timeout for one command is not enough for a pipeline: killing one process can
leave its children running. On POSIX, all job stages use one process group. On
cancellation, the runtime signals the group, waits for its children, and uses
`SIGKILL` after a short grace period. The group is configured in order: the first
actually started stage becomes the leader and later stages join its PGID.

Cancellation has two levels: `SIGTERM` after timeout or cancellation, then
`SIGKILL` after the grace period. The escalation mechanism is closed on every
return path so the safety logic does not leak goroutines.

Process groups are also required for foreground signal delivery to a pipeline.
Full terminal job control remains outside the current runtime. See [Linux
process groups](https://man7.org/linux/man-pages/man7/credentials.7.html).

## TTY and job control

File-oriented DevOps scripts can inherit stdin/stdout/stderr. A full interactive
shell additionally needs a controlling terminal, foreground process groups,
`SIGINT`/`SIGTSTP`/`SIGCONT`/`SIGWINCH`, stopped jobs, and PTY support for programs
that change behavior without a terminal.

This is a separate future area. yesh does not promise Bash-compatible `fg`,
`bg`, or Ctrl-Z behavior.

## Redirections

Redirection is part of the abstract syntax tree (AST), not textual insertion.
The runtime explicitly decides who opens the file, with which flags and mode,
and in what order stdin, stdout, and stderr are applied, who receives an open
error, and when the file descriptor is closed.

It does not make redirection privileged because a command contains `sudo`.
`sudo command > file` and `echo data | sudo tee file` remain different
operations.

## Exit status and signals

A process result preserves at least:

```text
started
exit_code
signal
timed_out
stderr
duration
```

A signal termination must not be reduced to an ordinary exit code.

## Environment and executable lookup

The environment is passed explicitly and must not be confused with yesh
variables. `SSH_AUTH_SOCK`, `PATH`, `HOME`, and `TERM` have operational meaning;
`sudo` may remove part of the environment according to policy.

External commands are started as `argv`, without a system shell, globbing, or
word splitting. A controlled `PATH` and environment policy for privileged
execution is still release work. Command lookup is not itself a safety proof,
and an unexpected `./program` must not silently replace a reviewed command.

Secrets are not automatically hidden: a value passed as an argument can appear
in `argv`, `--plan` can display the expanded argument, and an external-command
error can appear in bounded `stderr`. Keep secrets in an external store or an
SSH agent, and do not put them in scripts that are planned or logged.

### Using an SSH agent

Run these commands in the terminal that will start yesh, not inside a yesh
script:

```bash
eval "$(ssh-agent -s)"
ssh-add -t 1h ~/.ssh/id_ed25519
ssh-add -l
```

Before loading a private key, check that the `.ssh` directory and the key belong
to the user who will run yesh, and that the key is not readable by group or
other users:

```bash
stat -c '%A %U:%G %n' ~/.ssh ~/.ssh/id_ed25519
chmod 700 ~/.ssh
chmod 600 ~/.ssh/id_ed25519
```

Do not apply `chown` recursively without first confirming the exact owner and
path. OpenSSH may ignore a private-key file that is accessible to other users.
The agent does not make an incorrectly owned or exposed file safe.

`ssh-agent` holds client private-key identities in memory. `ssh-add` loads a
key after its passphrase is entered, and `ssh-add -l` lists fingerprints of
the identities currently held by that agent. The agent is selected through
`SSH_AUTH_SOCK`; starting a second agent in another shell does not populate the
first one. If an existing desktop or WSL agent already provides
`SSH_AUTH_SOCK`, use it instead of starting another agent.

Protect the agent socket: root or another process of the same user can use it
to request signatures. Do not forward the agent with `ssh -A` unless the
intermediate host is trusted and the extra hop is necessary.

Check the connection without an interactive prompt and require a previously
verified host key:

```bash
ssh -o BatchMode=yes \
    -o ConnectTimeout=5 \
    -o StrictHostKeyChecking=yes \
    -p 22 user@host true
```

`scp` uses the same agent when run in that environment. yesh does not read the
private key or manage the agent; it passes the explicit SSH/SCP arguments to
the external program. `--plan` can show those arguments but cannot prove that
the agent, host key, network, or remote account will work at execution time.
See the [OpenSSH `ssh-agent(1)` manual](https://man.openbsd.org/ssh-agent.1)
and [`ssh-add(1)` manual](https://man.openbsd.org/man1/ssh-add.1).

## What yesh deliberately changes

- implicit word splitting is absent;
- pipeline failure is checked across all stages;
- infinite loops and background jobs are rejected in the base mode;
- command substitution and nested shells are rejected;
- yesh does not promise to scrub secrets from `argv` or `stderr`;
- ordinary scripts do not require a PTY.

## What the test corpus covers

| Layer | What is checked | What it does not prove |
| --- | --- | --- |
| parser and runtime | string/list boundaries, `$name`, conditions, loops, errors, and `--plan` | the meaning or safety of every external command |
| POSIX processes | pipelines, `SIGPIPE`, process groups, cancellation, timeouts, child waiting, and redirects | TTY/PTY behavior or every kernel |
| Linux practical workflows | `rsync`, `rsnapshot`, `systemd`, journals, `/proc`, storage, initramfs, SSH/SCP, and network observation | production, power-loss durability, or another security policy |
| negative paths | missing inputs, wrong identities, held locks, partial transfers, timeouts, and service failures | recovery from every external outage |
| fuzz targets | generated inputs for lexing, parsing, value expansion, redirects, and runtime | safety of an arbitrary external program |

Linux practical checks have run in CI, WSL2, separate Debian and Rocky Linux
fixtures, loopback/ext4 fixtures, and an isolated KVM environment. A skipped
check caused by a missing external utility is not evidence that the integration
works.
