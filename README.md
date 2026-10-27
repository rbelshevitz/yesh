# yesh

![status: experimental](https://img.shields.io/badge/status-experimental-orange.svg)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A readable, safety-first Unix shell for DevOps and system administration.

yesh is an experiment: Unix commands remain Unix commands, while the shell
language around them is intentionally small and readable.

The first prototype supports:

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

yesh is a language sketch focused on readable DevOps and administration scripts.
The examples and design documents are the source of truth for its semantics.

Read the local manual with `man -l man/yesh.1`
(Russian: `man -l man/ru/yesh.1`). See
[pipeline safety](docs/pipeline-safety.ru.md), [cron](docs/cron.ru.md),
[package manager design](docs/packages.ru.md), and
[POSIX integration checks](docs/linux-integration.ru.md).
Design notes cover [syntax and declarative operations](docs/syntax-review.ru.md)
and [file permissions](docs/permissions.ru.md).
Runnable [practical examples](docs/practical-examples.ru.md) cover archives,
rsync, rsnapshot, configuration changes and cancelled jobs.
