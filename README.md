# yesh

![Go](https://img.shields.io/badge/Go-1.21-00ADD8.svg)
[![CI](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

yesh is a readable language for Linux automation and preflight checks.

> Bash without the historical syntax complexity.

yesh is a language for bounded Linux operations where it is clear what will run,
what will change, and how the result will be checked. Commands remain ordinary
Linux commands, while the script makes its preconditions, order of operations,
and failure boundary clear. yesh does not replace Bash or Ansible. It serves a
different purpose: describing and running small Linux operations where mistakes
can be costly. The plan is visible before execution and the result can be checked
afterwards. For these scenarios, yesh is easier to read and review than an
equivalent Bash script.

## What it is for

yesh targets small administrative scripts: backups, configuration publication,
service checks, diagnostics, and individual CI/CD steps. It is not an interactive
shell or a fleet-management system.

Its niche is a script of roughly 100–200 lines that is becoming difficult to
read in Bash, while Ansible or a separate application would be excessive.

    if grep -q '^mode=production$' config.next:
        install -m 0640 -- config.next /etc/myapp.conf.next
        publish /etc/myapp.conf.next to /etc/myapp.conf
        service restart myapp.service until active within 10s

External commands stay visible, arguments are not rebuilt into a shell string,
and `--plan` shows the order before execution.

The value of yesh is not measured by file length or by the number of built-in
operations. For a bounded scenario, the useful questions are whether the
arguments can be inspected before execution, the failure path is understandable,
the previous state is protected, and a report can be reproduced from saved
observations. These are criteria for individual high-consequence scenarios, not
a claim that yesh is better than Bash for every task.

See the [careful Bash baseline](examples/bash-rsync-incremental-baseline.sh)
next to the matching [yesh scenario](examples/rsync-incremental.yesh). It is
not a caricature: it makes the manual bookkeeping around `rsync` visible so
the difference in readability and review can be judged on one operation.

## Quick start

Go 1.21 or newer is required; Linux is required for Linux-specific features:

    go build -trimpath -o yesh ./cmd/yesh
    ./yesh examples/hello.yesh
    ./yesh --plan examples/hello.yesh

Start with the [quick start](docs/quickstart.md), then read the
[syntax guide](docs/syntax-guide.md) and [practical examples](docs/practical-examples.md).

Every file in `examples/` is checked for parsing. Some examples require
`YESH_*` environment variables, external tools, a running service, SSH, or a
separate fixture; without those conditions they stop before taking action.
That is an environment boundary, not a syntax error.

## Resource-constrained Linux systems

yesh supports read-only service checks under systemd and OpenRC: platform,
supervisor, PID, cgroup, process, and finite resource observations. Missing host
data is reported as unavailable; this is not continuous monitoring. See the
[OpenRC example](examples/openrc-auto-resource-check.yesh) and the
[resource-constrained systems note](docs/underthehood/resource-constrained-os.md).

Commands remain external: yesh does not rewrite `rsync`, `mount`, `sed`, `awk`,
package managers, or the shell. Observations prove only the environment where
they were collected; a skipped dependency is not evidence of support.

## Documentation

- [quick start](docs/quickstart.md);
- [language guide](docs/syntax-guide.md);
- [practical examples](docs/practical-examples.md);
- [systemd and resource measurement](docs/systemd.md);
- [OpenRC and resource-constrained systems](docs/underthehood/resource-constrained-os.md);
- [Bash comparison corpus](docs/underthehood/bash-corpus.md);
- [security boundaries](SAFETY.md);
- [language design](DESIGN.md);
- [current TODO](docs/TODO.md);
- [manual page](man/yesh.1).

Detailed Linux test results are in the
[under-the-hood section](docs/underthehood/README.md). The project is under
active development; the current stated platform is Linux/amd64.

[Русский README](README.ru.md) · [README in Hebrew](README.he.md)

## Verify from a checkout

    GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go test ./...
    GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go vet ./...
    GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go build -trimpath ./cmd/yesh

License: MIT.
