# yesh

![Go](https://img.shields.io/badge/Go-1.21-00ADD8.svg)
[![CI](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

yesh is readable Linux automation you can explain.

> Make the checks, exact `argv`, order of operations, and failure boundary
> visible before a small administrative script changes a real node.

## The niche

yesh is for the awkward middle ground between a shell one-liner and a full
automation system: one Linux node, one bounded operation, several things that
must be checked, and a failure that must not be mistaken for success.

| If the task is... | Use... |
| --- | --- |
| one transparent command | the command itself or a small shell script |
| a fleet, inventory, or desired state | Ansible or an orchestrator |
| one host, several checks, a lock, visible commands, and a verified result | yesh |

For example, a static release should not become visible just because a copy
finished. The operator may need to validate the web-server configuration,
prevent two deploys, switch one name atomically, check HTTP, and restore the
previous name when the new release fails:

```yesh
with lock $lock:
    /usr/sbin/nginx -t
    publish $release to $release_final
    ln -s -- $release_final $current_next
    mv -T -- $current_next $current

    if curl --fail --silent --show-error --max-time 10 $health_url:
        echo "release is serving"
    else:
        ln -s -- $previous $current_next
        mv -T -- $current_next $current
        exit 4
```

The web server, `curl`, filesystem, and permissions still belong to the host.
yesh makes the order and the failure boundary reviewable; it does not become a
web server, a container runtime, or a privilege manager.

## See it run

```console
$ go build -trimpath -o yesh ./cmd/yesh
$ ./yesh --plan examples/hello.yesh
PLAN ["echo" "hello" "world"]
PLAN ["echo" "item:" "one"]
PLAN ["echo" "item:" "two"]
PLAN ["echo" "item:" "three"]
```

The plan is produced from the same parsed script that would run, without
starting external commands or opening redirections. For a real scenario, start
with the [static-release examples](docs/practical-examples.md#static-release-publication-with-openrc-and-systemd)
or the [incremental rsync example](examples/rsync-incremental.yesh).

If yesh is new to you, the separate TUI assistant offers a gentle first path
through it. It leads from a task choice to a small form, explains the checks,
shows the real script plan, and makes the saved file explicit. The assistant
does not run the generated script or hide its external commands; it is a guided
introduction through two bounded workflows: backups and read-only service
inspection.

Run it from a working tree:

    go run -tags navigator ./cmd/yesh-navigator

The assistant is built as a separate companion binary; the regular `yesh` binary
does not depend on it.

yesh is a language for bounded Linux operations where it is clear what will run,
what will change, and how the result will be checked. Commands remain ordinary
Linux commands, while the script makes its preconditions, order of operations,
and failure boundary clear. yesh does not replace Bash or Ansible. It serves a
different purpose: describing and running small Linux operations where mistakes
can be costly. The plan is visible before execution and the result can be checked
afterwards. For these scenarios, yesh is easier to read and review than an
equivalent Bash script.

yesh removes more than shell syntax noise. It makes recurring
protective code visible and reusable: preflight checks, failure boundaries,
argument inspection, locks, cleanup, planning, and result verification.
The result is a small executable operation card: less repeated scaffolding,
while the underlying Linux operations remain explicit.

## What it is for

yesh targets small administrative scripts: backups, configuration publication,
service checks, diagnostics, and individual CI/CD steps. It is not an interactive
shell or a fleet-management system.

For backups, `rsync` performs transfers and can restore a selected snapshot;
`rsnapshot` performs transfers and snapshot rotation. A restore command remains
external. yesh does not copy or restore data itself: it checks the destination,
controls order and locking, keeps incomplete results unpublished, and verifies
the outcome.

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
