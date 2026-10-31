# yesh syntax tutorial

> The English version is still being expanded. The Russian version is currently
> the complete tutorial.

yesh is a small language for Linux scripts where you need to see the order of
actions, the arguments passed to external commands, and what happens when a
command fails.

This tutorial is not a complete language reference. It starts with a safe path
check and then introduces the rules needed to read and write small
administrative scripts. For the complete guide, see the
[Russian syntax guide](syntax-guide.ru.md).

## The core idea

yesh does not turn a command into one shell string. It keeps the program name
and each argument separate, then passes that list directly to the external
program. In Unix terminology, this list is an argument vector (`argv`): the
first item is the program name and the remaining items are its arguments.

For example:

```yesh
install -m 0640 -- config.next /etc/myapp.conf.next
```

The external `install` program receives `-m`, `0640`, `--`, and the two paths as
separate arguments. yesh checks the expanded list and its boundaries; it does
not decide what `install` means or whether the operation has sufficient
permissions.

## Who this is for

This tutorial is for a Linux administrator who can run ordinary commands and
read their documentation, but does not want to learn Bash's historical syntax.
You should know what files, exit codes, and simple pipelines are.

Allow about 25–35 minutes for a first pass. By the end, you should be able to
inspect a plan, explain the arguments passed to an external command, and
predict what happens when a step fails.

## Before you start

Build yesh from the repository root:

```text
go build -trimpath -o yesh ./cmd/yesh
```

The first exercises do not require administrator privileges, a running systemd
instance, network access, or special hardware. You need Go to build yesh and
the Linux commands used by each example. Some later examples use
`systemctl`; you can read those examples and inspect them with `--plan` even
without systemd.

Comments start with `#` and continue to the end of the line. They describe the
purpose or limits of an example; they are not executed:

```yesh
# This comment does not run a command.
echo ready
```

## 1. First script

Start with a script that only checks whether a path exists. Save it as
`check-source.yesh`:

```yesh
source = "source"

if exists $source:
    echo "source is ready"
else:
    echo "source is missing"
    exit 2
```

Run it from a directory that contains `source`:

```text
./yesh check-source.yesh
```

If the path exists, the script prints `source is ready` and exits successfully.
If it does not, the script prints `source is missing` and returns code `2`.
The script does not change anything.

`exists` checks whether the path exists now. It does not promise that the
path will remain accessible later. A permission error or symlink loop is an
execution error, not a false result. Create or remove the path and check both
normal cases.

Before running the script, you can inspect its plan:

```text
./yesh --plan check-source.yesh
```

The plan shows both branches but does not run commands or open redirections.

## 2. Literal and variable

A word without `$` remains text:

```yesh
unit = nginx.service

echo unit
echo $unit
echo "$unit"
```

The output is:

```text
unit
nginx.service
$unit
```

The rule is simple: expand a variable only with `$name`. `${name}` is not
supported, and yesh does not split one string into words implicitly.

Double quotes do not expand variables in yesh. Therefore `"$unit"` is the
literal string `$unit`. Write `$unit` without quotes when you want the value;
yesh already keeps it as one argument.

An unknown variable is an error, not an empty string:

```yesh
echo $missing
```

## 3. Strings and lists

A string remains one value. A list expands into separate arguments:

```yesh
message = "backup finished"
hosts = [web-1, web-2]

echo $message
for host in $hosts:
    echo $host
```

Quotes keep a value together. Single quotes preserve their text. Double quotes
support Go escape sequences such as `\n` and `\t`. A comma inside quotes does
not split a list item.

Do not add quotes around a variable expansion to preserve its boundaries. The
language already knows the boundaries. Use quotes for the text itself.

## 4. Conditions and blocks

A block starts after `:` and uses indentation:

```yesh
config = "/etc/my-service.conf"

if exists $config:
    echo "configuration exists"
else:
    echo "configuration is missing"
    exit 2
```

`exists` returns false when the path is absent. Permission errors, symlink
loops, and invalid path values stop the script.

You can also use an external command as a condition:

```yesh
if systemctl is-active --quiet -- nginx.service:
    echo "nginx is active"
else:
    echo "nginx is not active"
    exit 1
```

If a command starts and returns a non-zero code, yesh takes the `else` branch.
If it cannot start, is cancelled, is killed by a signal, or exceeds its time
limit, yesh reports an execution error instead.

## 5. Finite loops

`for` iterates over a finite list:

```yesh
services = [nginx.service, sshd.service]

for service in $services:
    systemctl is-active --quiet -- $service
    echo $service "is active"
```

If one service check fails, yesh stops before the next item. `while` is not
supported, so the base language has no unbounded loop.

If a failed check is expected, put the command in an `if` and decide explicitly
what the result means. The loop does not hide command failures.

## 6. Pipelines and redirections

Linux commands remain visible:

```yesh
ps -eo pid=,ppid=,user=,rss=,stat=,comm= | sort -k 4,4nr > processes.txt
```

yesh starts each stage directly and passes exact arguments to it. It does not
reparse a constructed command through a shell.

If any stage fails, the whole pipeline fails. An input redirection `<` is
allowed on the first stage; `>` and `>>` are allowed on the last stage.
`--plan` shows the expanded arguments and paths without running commands or
opening files.

Before starting the pipeline, yesh parses it and checks its limits. If the
pipeline form is invalid, no stage starts. This does not make execution
atomic: an external command can write a partial result and then fail. Use a
temporary path and `publish` only after the command succeeds.

For example, replace `>` in the script above with the unsupported `2>`.
yesh should reject the redirection before starting the first stage.

## 7. Explicit exit

```yesh
code = 7
exit $code
echo "this line does not run"
```

`exit` ends the entire script with a decimal code from `0` through `255`. It is
a yesh statement, not an external command or a pipeline stage.

## 8. Exact arguments

A bare word is text, a variable expands explicitly, and a list supplies several
separate arguments:

```yesh
unit = nginx.service
options = [--quiet, --, $unit]

systemctl is-active $options
```

This passes three separate arguments to `systemctl` after the command name. A
variable value is not reparsed as yesh code or shell text.

This list is often called an argument vector, or `argv`: the first item is the
program name and the remaining items are its separate arguments. It is not one
shell command string, so spaces inside one value do not create extra arguments.

Spaces inside a value stay inside one argument:

```yesh
path = "backup files/source"
printf '%s\n' $path
```

An empty string is one argument. An empty list supplies no arguments. Expansion
errors are reported before the external command starts.

## 9. Environment variables

Use `environment` to read a value from the calling environment:

```yesh
source = environment YESH_BACKUP_SOURCE
target = environment YESH_BACKUP_TARGET

if exists $source:
    printf '%s\n' $source $target
else:
    echo "backup source is missing"
    exit 2
```

Save this as `environment-check.yesh` and run it with:

```text
YESH_BACKUP_SOURCE=source YESH_BACKUP_TARGET=backup ./yesh environment-check.yesh
```

The environment variable name is written without `$` because it is part of the
declaration. A missing or empty value is an immediate error; the script does
not start with a partially prepared set of values.

## 10. Regular expressions and `matches`

A regular expression is a value. `matches` is a separate condition:

```yesh
unit = "api-42"
pattern = regex '^api-[0-9]+$'

if $unit matches $pattern:
    echo "service name is valid"
else:
    echo "service name is invalid"
    exit 3
```

Patterns use Go RE2 syntax and are checked when assigned. The string
`"a matches b"` remains data; it does not become an operator.

## 11. Plans and error messages

Use `--plan` to inspect the action order and expanded arguments:

```text
./yesh --plan check-source.yesh
```

The plan does not run external commands or open redirected files. For an
external command stage, it shows the exact `argv` yesh would pass to the
program. A built-in yesh operation, such as `publish`, is shown as a planned
yesh action with its parameters, not as an external `argv`. The plan cannot
guarantee that the network, filesystem, service, or external program will still
be available when the real run starts.

If a command starts and fails, yesh reports the file, line, and why it stopped.
An execution error is not an ordinary false condition, so adding `else`
does not handle it:

```yesh
if missing-command:
    echo "not reached"
else:
    echo "also not reached"
```

`else` is selected only when a command started and returned a non-zero code.

### What yesh can check

Separate what yesh checks from what the external system knows:

| yesh checks | The external system or command controls |
| --- | --- |
| action order and branching | network, service, and filesystem state |
| exact arguments for an external call | the utility's meaning and result |
| stopping after an error | changes that happened before the error |
| a plan without execution | conditions that may change between planning and running |

`--plan` lets you review the script's intent. It cannot promise that an
external command will succeed later.

## 12. What yesh does not do

For repeatable admin work, yesh has a few operations with small, clear rules:

```yesh
with lock /var/lib/yesh/backup.lock:
    rsync -a -- source/ temporary/
    publish temporary to current
```

`with lock` does not wait for the lock. It releases the lock on every exit path.
`publish` uses a same-filesystem rename; it does not silently copy across
filesystems. If the lock is busy, the body does not start. If `rsync` fails,
`publish` is not reached.

`publish` does not merge directories. In this example, `current` must be an
absent name or a replaceable symlink to the previous result.

yesh does not grant extra privileges, replace `rsync`, `systemctl`, or `tar`, or
act as a shell. Network, filesystem, and service state can change after a check.
The base language also does not include `cd`, functions, command substitution,
automatic `sudo`, or rollback of actions that already ran.

## 13. A complete publication workflow

Validate a prepared file before replacing the active one:

```yesh
staged = "active.conf.next"
active = "active.conf"

if exists $staged:
    grep -q '^mode=production$' $staged
    publish $staged to $active
    echo "configuration published"
else:
    echo "prepared configuration is missing"
    exit 2
```

The order is explicit: check that the candidate exists, validate its contents,
then rename it into place. If `grep` fails, `publish` is not reached. If the
candidate is missing, the external check does not run.

This still does not show that the application can use the new configuration.
That requires an application-specific check and, when needed, a service
operation followed by a check for the required state.

## 13a. Practical example: rsnapshot

The same rules can guard an external `rsnapshot` interval. yesh does not parse
or rewrite `rsnapshot.conf`, and it does not implement rotation itself. Its
role is to check the prerequisites, show the test run, require explicit
confirmation, and verify the result.

Before reading the yesh script, remember the basic `rsnapshot` model:

- `snapshot_root` is the directory that contains the snapshot levels;
- `retain daily 2` keeps `daily.0` as the newest level and `daily.1` as the
  previous one;
- unchanged files commonly share hard links between levels, so a snapshot is
  not an independent copy to edit in place;
- `no_create_root 1` prevents the tool from silently creating the root. The
  root must already exist and be the expected mounted filesystem;
- `backup /path/./ local/` sets the source-tree boundary: the path before
  `/./` is not reproduced inside the snapshot;
- `rsnapshot -t` shows the selected interval's actions, but it does not replace
  `configtest` or prove that the transfer will succeed.

`configtest` is rsnapshot's own configuration check: it validates the
configuration syntax and required programs, but it does not run a backup or
change snapshots. The `-t LEVEL` mode comes next: it shows the intended
commands for the selected level without rotating snapshots, but it also does
not prove that a future transfer will succeed.

A small temporary configuration might look like this. Use tabs between fields
in an actual `rsnapshot.conf`:

```text
config_version	1.2
snapshot_root	/backup/snapshots/
no_create_root	1
retain	daily	2
lockfile	/tmp/rsnapshot.pid
backup	/srv/data/./	local/
```

The complete yesh example is
[`examples/rsnapshot-apply.yesh`](../examples/rsnapshot-apply.yesh). Its
inputs are explicit environment values:

```yesh
config = environment YESH_RSNAPSHOT_CONFIG
level = environment YESH_RSNAPSHOT_LEVEL
root = environment YESH_RSNAPSHOT_ROOT
mount_source = environment YESH_RSNAPSHOT_SOURCE
lock = environment YESH_RSNAPSHOT_LOCK
report = environment YESH_RSNAPSHOT_REPORT
published = environment YESH_RSNAPSHOT_PUBLISHED
confirm = environment YESH_RSNAPSHOT_CONFIRM
```

The important order is:

```yesh
if exists $config:
    if exists $root:
        if findmnt --source $mount_source --target $root:
            if free_space $root at_least 20MiB:
                rsnapshot -c $config configtest
                rsnapshot -c $config -t $level | tee $report
                if exists $confirm:
                    rsnapshot -c $config $level
                    if exists $published:
                        if exists $lock:
                            echo "rsnapshot lockfile remains after successful interval"
                            exit 6
                        else:
                            echo "rsnapshot interval completed"
```

The full file also reports a missing root, an unmounted destination, and low
free space. The order is readable from top to bottom: check the configuration,
root, mount, and free space; run `configtest`; show and save the interval's
test-mode report through `tee`; flag known deletion-related rsync options;
require a pre-created confirmation file; apply the interval; then check for
the expected `daily.0` and a released lock file.

This is what yesh simplifies: the preflight order, the boundary between test
mode and the changing command, the exact visible `rsnapshot` arguments, and
the failure path are ordinary blocks instead of Bash-specific control-flow
machinery. In `--plan`, yesh shows both branches without running `findmnt` or
`rsnapshot` and without opening redirected files.

The Bash idea would be:

```bash
rsnapshot -c "$config" configtest || exit 1
rsnapshot -c "$config" -t "$level" | tee "$report" || exit 1
test -e "$confirm" || exit 4
rsnapshot -c "$config" "$level" || exit 1
test -d "$published" || exit 6
test ! -e "$lock" || exit 6
```

The rule is: test mode and explicit confirmation come before the changing
operation; a missing snapshot or a remaining lock is an error. yesh does not
roll back a rotation that has already completed and does not delete a
quarantine.

The full disposable Real-World Example is
[`yesh-rsnapshot-loopback-rwe.sh`](../scripts/yesh-rsnapshot-loopback-rwe.sh).
It uses an ext4 loopback device and checks two rotations, restore, hard links,
an invalid configuration, missing confirmation, a held lock, and resource
cleanup. It does not use real disks or `/etc/rsnapshot.conf`.

## 14. Reading an unfamiliar script

Ask five questions before you run a yesh script:

1. Which values does it define, and where does it expand them with `$`?
2. Which external commands does it run, and which arguments do they receive?
3. Which files, services, or other objects can it change?
4. What happens after a non-zero code, a missing path, or a permission error?
5. What will `--plan` show, and which external conditions can still change?

If the script does not answer one of these questions from its text and the
language rules, stop and clarify it before running it.

## 15. Pre-run checklist

Before you run it for real, check that:

- required environment variables are set and non-empty;
- every value is expanded explicitly with `$name`;
- each external command receives the arguments you expect;
- commands that change data use temporary paths until publication;
- an external failure cannot produce a false success message;
- `--plan` shows the same order of actions you are about to run.

If the script needs `sudo`, run yesh in an environment that already has the
required privileges, then inspect the plan and the real run there. yesh does
not add privileges by itself.

## 16. Exercise

Write `publish-report.yesh`. It should read these environment variables:
`YESH_REPORT_SOURCE`, `YESH_REPORT_TEMP`, and `YESH_REPORT_CURRENT`.

The script must:

1. check that the source file exists;
2. create a report at the temporary path with an external command;
3. avoid publication if that command fails;
4. publish the temporary result only after success;
5. return code `2` when the source is missing;
6. show a plan without running commands or changing files.

Test three cases: success, a missing source, and a failing external command. For
the failure case, decide what should remain at the temporary path. Confirm that
the previous published file is unchanged and that the script does not print a
false success message.

Use only existing yesh constructs. If the solution seems to need a new built-in
operation, first check whether ordinary Linux commands can express the workflow.

## 17. When something goes wrong

- **`yesh: command not found`** — build the binary or provide its full path. Use
  `./yesh` when running from the repository root.
- **`environment NAME is missing or empty`** — set the required environment
  variable and run the script again. An empty value is not valid input.
- **`undefined variable $name`** — check the name and expand it explicitly with
  `$name`. Quotes do not enable variable expansion.
- **parse error with a line number** — fix the syntax in the whole file first.
  yesh does not execute a partially parsed script.
- **external command failure** — run the command separately with the same
  arguments. Check permissions, paths, and system state. `else` handles only a
  non-zero code from a command that actually started.
- **permission denied** — do not add `sudo` inside the script automatically.
  Decide which environment should run the script, then inspect the plan and the
  normal run there.

## 18. Where to go next

After this tutorial, see:

- the [quickstart](quickstart.md) for a shorter first run;
- [Practical Examples](practical-examples.md) for complete administrative
  workflows and their boundaries;
- [pipeline safety](pipeline-safety.md) for pipelines, redirections, and process
  cancellation;
- the [language manual](../man/yesh-language.7) for the complete syntax.

For a selected-snapshot restore workflow, see
[`rsync-selected-restore.yesh`](../examples/rsync-selected-restore.yesh) and its
test fixture [`scripts/yesh-selected-restore-rwe.sh`](../scripts/yesh-selected-restore-rwe.sh).
