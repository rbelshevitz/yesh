# yesh

![status: experimental](https://img.shields.io/badge/status-experimental-orange.svg)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg)
[![CI](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A readable, safety-first language for Unix automation and preflight checks.

[Русская версия README](README.ru.md)

yesh is an experiment: POSIX commands and external Linux administration tools
remain ordinary commands, while the shell language around them is intentionally
small and readable.

> **Bash without the historical syntax complexity.**
>
> Read a yesh script from top to bottom. Strings stay strings, lists stay
> lists, commands stay external commands, and failures do not disappear
> silently.

![A system administrator having a Eureka moment in a data center](docs/yesh-eureka.png)

## Bash, but readable

| Bash | yesh |
| --- | --- |
| `name=world; echo "$name"` | `name = world`<br>`echo $name` |
| `items=(one two three)` | `items = [one, two, three]` |
| `if [ -e "$path" ]; then ... fi` | `if exists $path:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`...` |
| `for item in "${items[@]}"; do ... done` | `for item in $items:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`...` |
| `set -euo pipefail` | failures are errors by default |

## Why yesh is easier to read than Bash

yesh is not trying to replace Unix tools. It removes the shell-language traps
that make small Bash administration scripts difficult to read and easy to get
wrong:

- **Too many ways to express the same thing.** Bash conditions and arithmetic
  use several historical syntaxes with different rules. yesh keeps one clear
  block form: `if condition:` and `for item in $list:`.
- **Quoting and word-splitting traps.** In Bash, an unquoted expansion can be
  split into words or interpreted as a glob. In yesh, a string remains one
  argument and a list is expanded only as a list.
- **Ambiguous variable references.** yesh uses one explicit `$name` form for
  variable expansion; bare words remain literal, and there is no second
  `${...}` spelling to remember.
- **Errors hidden behind boilerplate.** A yesh command failure is an error by
  default; scripts do not need `set -euo pipefail` just to state their basic
  failure policy.
- **Commands reinterpreted as shell text.** yesh passes command arguments
  directly through its Go runtime instead of reparsing a constructed command
  string through another shell.
- **Scripts that are short but hard to review.** Indentation, finite lists,
  explicit branches, and visible exit paths make the operational intent clear
  to a reader who did not write the script.

The result is a small Go-based language for scripts that should be easy to
write and easy to review.

The current prototype supports:

- assignments such as `name = world` and `items = [one, two]`;
- `if condition:` and `for item in $items:` blocks with indentation;
- ordinary commands, pipes, and redirections;
- explicit `$name` variable expansion, with bare words kept literal;
- command failures that stop the script and report the source line;
- a single `exists $path` predicate, regex conditions, and explicit `exit code`.

## What works today

yesh is already useful for small read-only checks and guarded administration
steps built from ordinary POSIX commands and external Linux tools:

- service checks through ordinary `systemctl` commands;
- SSH health checks and SCP transfers with an explicit port, timeout, batch
  mode, and host-key policy;
- mount, filesystem, UUID/PARTUUID, and free-space preflight checks;
- rsync preview and copy examples without implicit `--delete`;
- Linux process snapshots by exact kernel name, including PID, UID, state, RSS,
  process group, session, and cgroup;
- readable RSS totals, maximums, averages, and budget checks.

The `--plan` mode shows the exact argument vector (`argv`) that each command
stage would receive, including expanded lists and external conditions. It does
not start commands or open redirected output files. For example:

```text
PLAN ["scp" "-P" "2828" "-o" "BatchMode=yes" "-o" "ConnectTimeout=5" "--" "backup.tar.gz" "user@host:/srv/backup.tar.gz"]
```

This makes the real operation reviewable before execution.

## Build and run

Build and run the Go binary:

```text
go build -trimpath -o yesh ./cmd/yesh
./yesh examples/hello.yesh
```

Start with the [English quickstart](docs/quickstart.md) for a Bash-to-yesh
comparison and the rules behind the syntax.

See the [English roadmap](docs/TODO.md) for current project priorities.

## Documentation and scope

yesh is a language sketch focused on readable system-administration and
operational scripts.
The examples and design documents are the source of truth for its semantics.

### Where yesh fits beside Ansible

yesh is not a replacement for Ansible. Ansible is an orchestration system with
inventory, modules, roles, and a desired-state model for many hosts. yesh is a
small Unix automation language for transparent local scripts, runbooks,
preflight checks, and command-level steps inside larger automation.
It keeps `ssh`, `rsync`, `systemctl`, and other tools visible instead of hiding
them behind a new module model. Use Ansible for fleet orchestration and
idempotent desired state; use yesh when a Bash-like Unix workflow needs clearer
syntax, explicit arguments, and reviewable execution.

## Core execution model

Values keep their type: a string stays one argument and a list expands to
multiple arguments explicitly. Commands are executed directly by the Go
runtime, without reparsing a command string through another shell. yesh is a
small language for readable Unix automation.

Read the English local manual with `man -l man/yesh.1`.
See [predictable pipelines](docs/pipeline-safety.md) for the execution and
redirect safety rules.
See [execution safety](SAFETY.md) for the security boundary and operational
rules; this is not a sandbox for untrusted scripts.
The core design notes are [control flow](docs/control-flow.md),
[design principles](DESIGN.md),
[storage design](docs/storage.md), [remote operations](docs/remote-ops.md),
[processes and text](docs/processes-and-text.md), and
[systemd integration](docs/systemd.md).
Runnable [English practical examples](docs/practical-examples.md) cover archives,
rsync, rsnapshot, configuration changes, and read-only administration.

The runnable examples also include read-only process, journal, mount and SSH
health checks in `examples/`.

Russian project documents and the Russian manual are linked from
[README.ru.md](README.ru.md); they are not presented as English documentation.

## Comparison with Bash

[Bash](https://www.gnu.org/software/bash/) is the GNU Project's **Bourne Again
SHell**. It was originally developed by [Brian Fox](https://www.gnu.org/people/people.en.html)
and is currently maintained by Chet Ramey.

yesh is an independent implementation written in Go, with a smaller syntax
around familiar POSIX/Linux commands. It takes inspiration from POSIX/Linux
shell workflows, not from copying or embedding Bash source code. Its purpose
is to make everyday administration scripts easier to read and safer to review.
