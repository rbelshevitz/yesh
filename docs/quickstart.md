# yesh quickstart

yesh is a small Go-based shell for readable system-administration and
operational scripts.

> **Bash without the historical syntax complexity.**

This page uses only syntax supported by the current runtime. It is not a
promise of future builtins or declarative configuration features.

## One small health check

The task: check a list of services and stop if any active-state check fails.
This needs an accessible systemd manager and installed services; replace
`nginx` and `sshd` with the unit names used on your machine.

### Bash

```bash
services=(nginx sshd)

systemctl show --property=Version >/dev/null || exit 1

for service in "${services[@]}"; do
    if systemctl is-active --quiet -- "$service"; then
        echo "$service is active"
    else
        echo "$service active check failed"
        exit 1
    fi
done
```

### yesh

```yesh
services = [nginx, sshd]

systemctl show --property=Version > /dev/null

for service in $services:
    if systemctl is-active --quiet -- $service:
        echo $service "is active"
    else:
        echo $service "active check failed"
        exit 1
```

The standalone command first checks access to the systemd manager. Its
failure stops the script before any service-state message. If access is
lost later, `active check failed` reports an unsuccessful check rather than
claiming the service is stopped. Diagnostic details remain on stderr.

The yesh version reads as a sequence of ordinary sentences:

1. define a list;
2. check access to systemd;
3. for each service;
4. check the command result;
5. report the result;
6. stop explicitly on failure.

There is no `$service`, `${services[@]}`, `[[ ... ]]`, `then`, `fi`, `do`,
`done`, or `set -euo pipefail` to decode.

## What yesh changes

### Values keep their type

```yesh
services = [nginx, sshd]
for service in $services:
    echo $service
```

`services` is a list and `service` is one string value. A list is expanded by
the language as arguments; a string remains one argument. There is no implicit
word splitting that can turn spaces inside a value into extra arguments.

### Commands remain Unix commands

```yesh
ps -eo pid=,ppid=,user=,rss=,stat=,comm= | sort -k 4,4nr
```

Pipelines, redirections, and command arguments keep their familiar Unix form.
yesh executes the command arguments directly through its Go runtime; it does
not reparse the whole line through another shell.

`sort` consumes the entire input and sorts rows by RSS in descending order.
A reader that exits early, such as `head`, may cause the writer to receive
SIGPIPE; yesh treats a failure in any stage as a pipeline failure. This first
example therefore avoids early reader termination.

### Errors are visible by default

A started command returning zero makes the condition true; a normal nonzero
exit status makes it false. A missing executable, startup failure, signal,
cancellation or timeout remains an execution error and does not enter `else`.
If an ordinary command fails outside a condition, yesh stops the script and
reports the source location. No `set -e` ritual is needed.

An explicit exit remains explicit:

```yesh
if exists /etc/my-service.conf:
    echo "configuration exists"
else:
    echo "configuration is missing"
    exit 2
```

`exists path` is a yesh predicate. It is not a hidden shell test and does not
claim that the file will remain accessible after the check.

## Run it

Build the interpreter from the repository root:

```text
go build -trimpath -o yesh ./cmd/yesh
```

Run a self-contained example that does not need systemd or installed services:

```text
./yesh examples/hello.yesh
```

On a systemd host, after choosing your service names, run
`./yesh examples/service-check.yesh`. This checks state; it does not start services.

Preview the service script in **dry-run** mode (no systemd access required):
inspect expanded commands and branches without running external programs:

```text
./yesh --plan examples/service-check.yesh
```

`--plan` is useful during review: it shows the argument vectors and planned
branches while avoiding external command execution. It previews the plan; it
does not run the script.

## The boundary

yesh is a small language for Unix automation, not Python with a command
shortcut. It deliberately keeps its language surface small:

- use Unix tools such as `grep`, `sed`, `awk`, `ssh`, `systemctl`, and `rsync`;
- use yesh for readable control flow, typed strings/lists, direct arguments,
  bounded execution, and failure handling;
- do not expect unsupported shell features such as command substitution,
  `while`, implicit background jobs, or nested shell evaluation.

The design test is simple: if a new feature does not make an administrative
script easier to read, review, or safely repeat, it does not belong in the
small core.
