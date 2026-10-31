# yesh

![Go](https://img.shields.io/badge/Go-1.21-00ADD8.svg)
[![CI](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A readable language for Linux automation and preflight checks, with explicit
failure behavior.

[Русская версия README](README.ru.md)

yesh is an early-stage project: ordinary Unix utilities and external Linux
administration tools remain visible commands, while the language around them is
intentionally small and readable. yesh is a language and interpreter for Linux
automation, not an interactive shell or a Bash-compatible replacement. A yesh
script describes a sequence of actions, their arguments, and what to do when
one of them fails.

> **Development status:** yesh is under active development and is not yet ready
> to serve as a production safety boundary. The measures listed in
> [SAFETY.md](SAFETY.md) define the project's requirements; the current
> prototype does not yet satisfy every stable-release requirement. Treat it as
> reviewable tooling, not as a substitute for operator judgment or operating-
> system isolation.

> **Bash without the historical syntax complexity.**
>
> Read a yesh script from top to bottom. Strings stay strings, lists stay
> lists, commands stay external commands, and failures do not disappear
> silently.

![A system administrator having a Eureka moment in a data center](docs/yesh-eureka.png)

## The point

The useful difference appears at the boundary where a small Bash script starts
mixing validation, file replacement, service restart, and failure handling:

Bash:

```bash
set -e
if grep -q '^mode=production$' config.next; then
    install -m 0640 -- config.next /etc/myapp.conf.next
    mv -T /etc/myapp.conf.next /etc/myapp.conf
    systemctl restart myapp.service
    systemctl is-active --quiet myapp.service
fi
```

yesh:

```yesh
if grep -q '^mode=production$' config.next:
    install -m 0640 -- config.next /etc/myapp.conf.next
    publish /etc/myapp.conf.next to /etc/myapp.conf
    service restart myapp.service until active within 10s
```

The yesh version keeps validation, the publication boundary, and the service
postcondition visible. The external `install` command prepares the file and its
mode; `publish` uses a same-filesystem rename and does not silently fall back
to copying. The typed restart has a finite ten-second budget and checks that
the exact unit becomes active.

The corresponding `--plan` output makes the boundary explicit:

```text
PLAN IF ["grep" "-q" "^mode=production$" "config.next"]
PLAN ["install" "-m" "0640" "--" "config.next" "/etc/myapp.conf.next"]
PLAN publish "/etc/myapp.conf.next" to "/etc/myapp.conf"
PLAN ["systemctl" "restart" "myapp.service"] until active within 10s
```

The rule is simple: `PLAN [...]` is an external command's argument list
(`argv`); `PLAN publish ...` is a yesh operation with its own parameters.

## Try it in a minute

From a checkout with Go 1.21 or newer:

```text
go build -trimpath -o yesh ./cmd/yesh
./yesh examples/hello.yesh
./yesh --plan examples/hello.yesh
```

The second command runs a self-contained example. The third shows the same
script's expanded commands without starting an external command. For a
Linux-specific example, see [`service-status.yesh`](examples/service-status.yesh);
it requires a running systemd manager and only observes one service.

| Bash | yesh |
| --- | --- |
| `name=world; echo "$name"` | `name = world`<br>`echo $name` |
| `items=(one two three)` | `items = [one, two, three]` |
| `if [ -e "$path" ]; then ... fi` | `if exists $path:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`...` |
| `for item in "${items[@]}"; do ... done` | `for item in $items:`<br>&nbsp;&nbsp;&nbsp;&nbsp;`...` |
| `set -euo pipefail` | failures are errors by default |

This table is an orientation, not the main proof that yesh is useful. The main
evidence is **Real-World Examples**: short administrative workflows where
the script has real preconditions, an external Linux tool, a failure path and
an observable result.

| Real-world workflow | Bash risk | What yesh makes reviewable |
| --- | --- | --- |
| incremental backup with `rsync` | quoting, accidental destructive flags, partial publication | explicit `argv`, lock, preflight, staging and publication boundary |
| service restart and verification | a successful restart command is mistaken for a healthy service | bounded restart plus an explicit postcondition |
| staged configuration update | invalid content replaces the active file | validate first, publish atomically, then inspect the result |
| SSH/SCP delivery | hidden host-key or timeout policy, unclear argument boundaries | visible connection policy and exact arguments |

A Real-World Example is successful only when it demonstrates a concrete
readability or safety advantage over the Bash version. A one-line wrapper around
an external command is useful as a smoke test, but is not by itself evidence for
yesh.

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

This example is not necessarily shorter than its
Bash equivalent. The benefit appears when a script grows: yesh keeps argument
boundaries, the publication step, the failure rule, and the reviewable `--plan`
visible instead of making them depend on shell quoting and conventions.

## What the prototype supports

The current prototype supports:

- assignments such as `name = world` and `items = [one, two]`;
- `if condition:` and `for item in $items:` blocks with indentation;
- Linux-scoped `with lock $path:` blocks for non-blocking advisory locks;
- `publish source to destination` for same-filesystem atomic rename;
- ordinary commands, pipes, and redirections;
- explicit `$name` variable expansion, with bare words kept literal;
- command failures that stop the script and report the source line;
- one `exists $path` predicate, regex conditions, and explicit `exit code`;
- typed Linux operations for `service status`, bounded `service wait`, restart
  with a postcondition, and bounded `journal` over
  `systemctl`/`journalctl`;
- `--plan` for expanded argv and branch inspection without running commands.

## What works today

yesh is already useful for small read-only checks and guarded administration
steps built from ordinary Unix utilities and external Linux tools:

- bounded service and journal checks through explicit typed operations over
  `systemctl`/`journalctl`, without hidden `sudo`;
- SSH health checks and SCP transfers with an explicit port, timeout, batch
  mode, and host-key policy;
- mount, filesystem, UUID/PARTUUID, and free-space preflight checks;
- rsync preview and copy examples without implicit `--delete`;
- Linux process snapshots by exact kernel name, including PID, UID, state, RSS,
  process group, session, and cgroup;
- readable RSS totals, maximums, averages, and budget checks.

The `--plan` mode shows the exact argument vector (`argv`) that each command
stage would receive, including expanded lists and external conditions. It does
not start external commands or open redirected output files. Built-in yesh
operations, such as `publish`, are shown as planned yesh actions with their
parameters, not as external `argv`. For example:

```text
PLAN ["scp" "-P" "2828" "-o" "BatchMode=yes" "-o" "ConnectTimeout=5" "--" "backup.tar.gz" "user@host:/srv/backup.tar.gz"]
```

This makes the real operation reviewable before execution.

## How operational reliability is established

Operational reliability is not a promise that a server will never fail. The
useful question is: “What happened, what changed, and what did not change when
the plan went wrong?” yesh is judged by that question: by failure behavior,
safe reruns, and whether a reviewer can see the boundary of the damage.

yesh does not make external commands or host configuration reliable by magic.
It makes preconditions, ordering, and failure boundaries visible, and we test
those properties in layers:

| Test layer | What it checks | What it does not prove |
| --- | --- | --- |
| parser/runtime tests | syntax, value boundaries, expansion, errors, pipelines, redirects, cancellation and `--plan` | Linux kernel or utility behavior |
| safety-contract tests | required preflight order, exact `argv`, forbidden destructive flags and no hidden shell reparse | that the real utility will accept every option |
| disposable fixture tests | backup/restore, publication, permissions, lock conflicts, reruns, partial failures and unchanged state after a failure | durability after power loss or production-scale load |
| Linux practical tests | real `rsync`, `rsnapshot`, `systemctl`, `journalctl`, `/proc`, mounts and initramfs tools where available | behavior on a different kernel, distro, filesystem or policy |
| negative-path tests | missing input, wrong identity, unavailable service, failed pipeline stage, timeout and cancelled job stop before the next side effect | recovery from every external outage |

The important assertion is usually not only “the command returned zero”. A
practical test also checks the resulting files, permissions, ordering, emitted
diagnostics, absence of an unpublished result, and release of locks or child
processes. Missing external tools are reported as skipped with the required
package; a skip is not evidence that the integration works. Tests that need
root, a real systemd manager, a device, SSH or a second UID are labelled as
environment-dependent rather than presented as portable proof.

The current evidence covers Linux behavior through Linux CI, disposable Linux
fixtures, and additional opt-in tests on a separate Debian host. Those tests
exercise rsync and rsnapshot, including restore, lock, hard-link,
partial-failure, and rerun cases. These results establish behavior in the
named environments; they do not establish universal compatibility, power-loss
durability, or production safety.

The runnable examples and their limitations are documented in
[Practical Examples](docs/practical-examples.md). A local verification run
uses workspace-local caches:

Every `examples/*.yesh` file is parsed by the test suite. Not every file runs in
every default test job: syntax-guide and quickstart examples are exercised by
named tests, while environment-dependent examples are covered by their Linux
tests or opt-in fixture scripts. Each such example must state its required
tools and environment.

```text
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go test ./...
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go test -race ./...
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go vet ./...
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go build -trimpath ./cmd/yesh
```

These checks support a bounded operational claim: yesh's own execution model
and the tested workflows behave predictably in the stated
environment. They are not a claim of universal correctness, crash durability,
security isolation, or safe execution of arbitrary external programs.

## Where yesh should help an administrator

The useful target is the awkward layer around mature Linux tools. The underlying
tool already knows how to copy, restart, inspect or archive; yesh should make the
preconditions, sequencing, failure policy and postcondition readable:

- backup and restore workflows around `rsync`, `rsnapshot`, `tar` and `pv`;
- service runbooks around `systemctl` and bounded `journalctl` checks;
- storage preflight around `findmnt`, `lsblk`, `blkid`, `df`, `du` and fstab,
  without silently mounting, changing disks or weakening permissions;
- SSH/SCP deployment checks with explicit host-key, timeout and publication
  rules;
- read-only diagnostics around `/proc`, cgroups, initramfs, permissions,
  cron and package-manager queries;
- guarded configuration publication: validate a staged file, publish it
  atomically, then verify the observable state.

The selection rule is deliberately narrow: add a typed operation only when the
same readability or safety problem appears in at least two real workflows.
Otherwise the Linux utility remains an ordinary visible command. yesh should
orchestrate difficult commands, not grow a parallel implementation of `rsync`,
`systemctl`, `mount`, `sed`, `awk` or a package manager.

## Platform and documentation

yesh targets Linux. Its examples use external tools and may additionally require a running systemd
manager, root, a mounted filesystem, or a disposable test fixture. The
repository does not claim support for non-Linux operating systems.

Start with the [English quickstart](docs/quickstart.md) for a Bash-to-yesh
comparison and the rules behind the syntax.

The [Russian syntax tutorial](docs/syntax-guide.ru.md) gives a step-by-step
path from the first script to a small administrative workflow.

The [English syntax tutorial](docs/syntax-guide.md) covers the same syntax and
examples. The Russian tutorial contains the fuller step-by-step narrative.

See the [English roadmap](docs/TODO.md) for current project priorities.

## Documentation and scope

yesh is an early-stage language focused on readable system administration and
operational scripts, not a general-purpose shell or an interactive command
environment.
The examples and design documents are the source of truth for its semantics.

### Where yesh fits beside Ansible

yesh is not “Ansible, but with a different syntax”, and it is not a replacement
for Ansible. Ansible is an orchestration system with inventory, modules, roles,
and a desired-state model for many hosts. yesh is a small Linux runbook and
preflight language for transparent local workflows and command-level steps
inside larger automation.
It keeps `ssh`, `rsync`, `systemctl`, and other tools visible instead of hiding
them behind a new module model. Use Ansible for fleet orchestration and
idempotent desired state; use yesh when a Bash-like Unix workflow needs clearer
syntax, explicit arguments, and reviewable execution.

The intended relationship is complementary: Ansible can select hosts and
coordinate a rollout, while a yesh script can perform or inspect one carefully
bounded Linux operation on a host. A yesh script may also be the reviewed
runbook used directly by an administrator when a full inventory and role model
would add more machinery than the task needs.

## Core execution model

Values keep their type: a string stays one argument and a list expands to
multiple arguments explicitly. Commands are executed directly by the Go
runtime, without reparsing a command string through another shell. yesh is a
small language for readable Linux automation. It is not a shell interface: it
does not provide an interactive prompt or interpret a script by passing it to
Bash. Its runtime starts external commands directly and provides the language's
own variables, control flow, pipelines, and error handling.

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
around familiar Unix/Linux commands. It takes inspiration from Unix/Linux shell
workflows, not from copying or embedding Bash source code. Its purpose
is to make everyday administration scripts easier to read and safer to review.
