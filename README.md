# yesh

![status: experimental](https://img.shields.io/badge/status-experimental-orange.svg)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg)
[![CI](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A readable, safety-first Unix shell for DevOps and system administration.

[Русская версия README](README.ru.md)

yesh is an experiment: Unix commands remain Unix commands, while the shell
language around them is intentionally small and readable.

> **Bash without the historical syntax complexity.**
>
> Read a yesh script from top to bottom. Strings stay strings, lists stay
> lists, commands stay Unix commands, and failures do not disappear silently.

![A DevOps engineer having a Eureka moment in a data center](docs/yesh-eureka.png)

## Bash, but readable

| Bash | yesh |
| --- | --- |
| `name=world; echo "$name"` | `name = world`<br>`echo name` |
| `items=(one two three)` | `items = [one, two, three]` |
| `if [ -e "$path" ]; then ... fi` | `if exists path:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`...` |
| `for item in "${items[@]}"; do ... done` | `for item in items:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`...` |
| `set -euo pipefail` | failures are errors by default |

## Why yesh is easier to read than Bash

yesh is not trying to replace Unix tools. It removes the shell-language traps
that make small Bash administration scripts difficult to read and easy to get
wrong:

- **Too many ways to express the same thing.** Bash conditions and arithmetic
  use several historical syntaxes with different rules. yesh keeps one clear
  block form: `if condition:` and `for item in list:`.
- **Quoting and word-splitting traps.** In Bash, an unquoted expansion can be
  split into words or interpreted as a glob. In yesh, a string remains one
  argument and a list is expanded only as a list.
- **Variables that disappear into punctuation.** yesh uses names directly in
  value positions, without requiring `$` or layers of `${...}` syntax.
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
- `if condition:` and `for item in items:` blocks with indentation;
- ordinary commands, pipes, and redirections;
- name-based variable expansion without `$`;
- command failures that stop the script and report the source line;
- a single `exists path` predicate, regex conditions, and explicit `exit code`.

Build and run the Go binary:

```text
go build -trimpath -o yesh ./cmd/yesh
./yesh examples/hello.yesh
```

Start with the [English quickstart](docs/quickstart.md) or
[Russian quickstart](docs/quickstart.ru.md) for a Bash-to-yesh comparison and
the rules behind the syntax.

See the [English roadmap](docs/TODO.md) or the
[detailed Russian plan](docs/TODO.ru.md) for current project priorities.

yesh is a language sketch focused on readable DevOps and administration scripts.
The examples and design documents are the source of truth for its semantics.

Values keep their type: a string stays one argument and a list expands to
multiple arguments explicitly. Commands are executed directly by the Go
runtime, without reparsing a command string through another shell. yesh is not
a Bash compatibility layer; it is a smaller language for readable Unix
automation.

Read the local manual with `man -l man/yesh.1`
(Russian: `man -l man/ru/yesh.1`). See
[pipeline safety](docs/pipeline-safety.ru.md), [cron](docs/cron.ru.md),
[package manager design](docs/packages.ru.md), and
[POSIX integration checks](docs/linux-integration.ru.md).
Design notes cover [syntax and declarative operations](docs/syntax-review.ru.md)
and [file permissions](docs/permissions.ru.md).
Runnable [practical examples](docs/practical-examples.ru.md) cover archives,
rsync, rsnapshot, configuration changes and cancelled jobs.

The runnable examples also include read-only process, journal, mount and SSH
health checks in `examples/`.

## Relationship to Bash

[Bash](https://www.gnu.org/software/bash/) is the GNU Project's **Bourne Again
SHell**. It was originally developed by [Brian Fox](https://www.gnu.org/people/people.en.html)
and is currently maintained by Chet Ramey.

yesh is an independent implementation written in Go, with its own smaller
syntax around familiar Unix commands. It takes inspiration from Unix shell
workflows, not from copying or embedding Bash source code. Its purpose is to
make everyday administration scripts easier to read and safer to review.
