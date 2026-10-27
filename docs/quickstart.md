# yesh quickstart

yesh is a small Go-based shell for readable DevOps and system-administration
scripts.

> **Bash without the historical syntax complexity.**

This page uses only syntax supported by the current runtime. It is not a
promise of future builtins or declarative configuration features.

## One small health check

The task: check a list of services and stop with an error if one is not
active.

### Bash

```bash
services=(nginx sshd)

for service in "${services[@]}"; do
    if systemctl is-active --quiet "$service"; then
        echo "$service is running"
    else
        echo "$service is not running"
        exit 1
    fi
done
```

### yesh

```yesh
services = [nginx, sshd]

for service in services:
    if systemctl is-active --quiet service:
        echo service "is running"
    else:
        echo service "is not running"
        exit 1
```

The yesh version reads as a sequence of ordinary sentences:

1. define a list;
2. for each service;
3. if the command succeeds;
4. report the result;
5. stop explicitly on failure.

There is no `$service`, `${services[@]}`, `[[ ... ]]`, `then`, `fi`, `do`,
`done`, or `set -euo pipefail` to decode.

## What yesh changes

### Values keep their type

```yesh
services = [nginx, sshd]
for service in services:
    echo service
```

`services` is a list and `service` is one string value. A list is expanded by
the language as arguments; a string remains one argument. There is no implicit
word splitting that can turn spaces inside a value into extra arguments.

### Commands remain Unix commands

```yesh
ps -eo pid=,ppid=,user=,rss=,stat=,comm= | head -n 25
```

Pipelines, redirections, and command arguments keep their familiar Unix form.
yesh executes the command arguments directly through its Go runtime; it does
not reparse the whole line through another shell.

### Errors are visible by default

If `systemctl` cannot start, or returns a failure status, the enclosing `if`
condition is false. If an ordinary command fails outside a condition, yesh
stops the script and reports the source location. No `set -e` ritual is needed.

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

Run a script normally:

```text
./yesh examples/service-check.yesh
```

Inspect expanded commands and branches without running them:

```text
./yesh --plan examples/service-check.yesh
```

`--plan` is useful during review: it shows the argument vectors and planned
branches while avoiding command execution.

## The boundary

yesh is not Bash compatibility mode and not Python with a command shortcut.
It deliberately has a smaller language:

- use Unix tools such as `grep`, `sed`, `awk`, `ssh`, `systemctl`, and `rsync`;
- use yesh for readable control flow, typed strings/lists, direct arguments,
  bounded execution, and failure handling;
- do not expect unsupported shell features such as command substitution,
  `while`, implicit background jobs, or nested shell evaluation.

The design test is simple: if a new feature does not make an administrative
script easier to read, review, or safely repeat, it does not belong in the
small core.
