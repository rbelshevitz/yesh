# yesh syntax tutorial

> This tutorial covers the core language and the Linux tasks verified by the
> test suite.

💡 To open the table of contents, press the three-bullet button in the
upper-right corner.

yesh is a small language for bounded Linux operations where you need to see what
will run, what may change, how the result will be checked, and what happens when
a command fails. For small administrative tasks where mistakes can be costly,
this makes the script easier to read and review than an equivalent Bash script.

This tutorial is not a complete language reference. It starts with a safe path
check and then introduces the rules needed to read and write small
administrative scripts. For exact syntax and implementation limits, see the
[language manual](../man/yesh-language.7).

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

### How yesh parses a file

yesh keeps a separate boundary between script text and execution:

```text
text → lexer and parser → AST → runtime → external Linux processes
```

The lexer first recognizes words, strings, lists, operators, and blocks while
respecting quoting. The parser then checks the grammar and builds an internal
tree (the AST). Only after parsing succeeds does the runtime expand values,
build the exact `argv`, choose a conditional branch, and start external
processes.

Four rules follow:

- a syntax error stops the file before the first command starts;
- the parser does not run commands, inspect host state, or silently repair
  ambiguous text;
- an ordinary word remains a literal, while a variable reference is explicit
  as `$name`;
- `--plan` uses the same parsed tree, but shows the possible action order and
  expanded arguments instead of starting commands.

For example, `echo service` and `echo $service` are different parser inputs:
the first passes the word `service`, while the second requires an existing
variable. An unknown `$service` is an error, not an empty string. The yesh
navigator uses the same parser: generated text is not accepted merely because
the form produced it.

## The experimental navigator

The navigator is an experimental terminal user interface (TUI): a
full-screen text interface for working in a terminal. It is a guide for a
first yesh script, not a second command executor. Its purpose is to help a
newcomer produce a readable first script
without having to remember the whole syntax up front, while keeping the
result visible and explainable. You choose an intention and fill in a form;
the navigator builds an ordinary yesh text file and passes that text to the
same parser used when a file is started from the terminal.

The TUI is still experimental. It is planned to move out of the main `yesh`
executable into a separate, optional `yesh-navigator` executable. Until that
extraction is complete, the tagged build is a development artifact; use the
regular `yesh` command to run scripts.

The workflow is:

1. fill in the form;
2. fix any errors reported by the parser;
3. after successful validation, open `Plan` to inspect the commands and their
   order;
4. save a new script file if you need one;
5. start it explicitly from the terminal only after reviewing the text and the
   plan yourself.

`Plan` is not a replacement for parsing. If the text fails to parse, plan
review and saving are blocked until the error is fixed. The navigator does not
run the generated script, guess a correction, or hide host-state checks behind
the form.

👀 Before saving, read the generated file and check its paths, service names,
permissions, and external commands. A successful parse proves only that yesh
understood the syntax and built a plan; it does not prove that the operation is
safe for a particular host or that you have the required privileges.

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

## Practical route

Read this tutorial as one operational workflow rather than a list of syntax
rules:

1. [incremental `rsync` snapshot](benchmark-rsync.md) — staging,
   `--link-dest`, publication, and failure without replacing the previous
   snapshot;
2. [running `rsnapshot`](rsnapshot.ru.md) — `configtest`, test mode, rotation
   review, and explicit confirmation;
3. [observing a service](systemd.md) — a finite snapshot, measurement, and
   summary;
4. [SSH keys](#ssh-keys-for-a-remote-source) — no password in the
   script, verified host keys, and no disabled host trust;
5. [preflight checks](#15-check-before-running) — plan output, failure, and
   the boundary of what was proved.

For each practical step, inspect `--plan` first, then run the positive stand,
then the negative case. A successful external command proves only its own
result; yesh separately checks whether the staged result may be published,
whether the previous snapshot stayed unchanged, and whether observations belong
to the same service invocation.

This route proves behaviour on a specific Linux stand. It does not prove power-
loss durability, arbitrary permissions, network availability, or behaviour on
every systemd and filesystem version.

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

## 5. Loops with a fixed number of repetitions

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
declaration. A missing or empty value is an error when that assignment is
evaluated. This does not mean that all required values were checked before the
first command: commands with side effects earlier in the script may already
have run. To guarantee a check before any change, read all required values at
the beginning or use a separate preflight script with no mutating commands.

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

A regular expression is for matching or extracting text. It does not define
the language structure and should not replace ordinary string, list, file, or
process operations. If an explicit comparison or a visible Linux command is
clearer, use that instead of adding another regex condition.

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
be available when the real run starts. It shows the possible execution
structure: conditional actions include both branches. It does not predict
which branch the real run will select.

This is not mainly a way to shorten a three-line script: a small Bash example
may be just as short. The benefit appears as the workflow grows, because yesh
keeps argument boundaries, publication steps, failure behavior, and the action
order visible for review.

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

### What yesh checks, and what remains outside it

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

### What problem does yesh solve?

Without this sequence, validation can happen after replacement, or a failed
check can leave the new file visible. yesh keeps “staged file -> content check
-> publication” in order and leaves the previous file unchanged when the
external check fails.

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

### What problem does yesh solve?

An operator can confuse configuration validation, test mode, and rotation, or
apply a change without reviewing the actual commands. yesh separates these
steps, requires explicit confirmation, and checks the expected result; `rsnapshot`
still performs the transfer and rotation.

The same rules can guard an external `rsnapshot` interval. yesh does not parse
or rewrite `rsnapshot.conf`, and it does not implement rotation itself. Its
role is to check the prerequisites, show the test run, require explicit
confirmation, and verify the result.

Before reading the yesh script, remember the basic `rsnapshot` model:

### Two different models: `rsync` and `rsnapshot`

`rsync` is primarily a transfer and reconciliation between one source and one
destination. It compares trees and brings the destination to the selected copy
policy; with an explicitly supplied `--delete`, extra destination data may be
removed. `--dry-run` previews one transfer without changing files, but it is
still only a forecast of that transfer.

`rsnapshot` is not a different transfer protocol. It organizes a sequence of
snapshots on top of `rsync`. It keeps named states such as `daily.0` and
`daily.1`, shifts and removes older levels according to `retain`, and then
performs the transfer for the selected interval. Unchanged files commonly use
hard links between levels, so every level looks like a full tree while storage
is mostly consumed by changes.

In the standard `rsnapshot` configuration, the test output commonly contains
`--delete` and `--delete-excluded`. These are flags passed to `rsync`: they do
not delete the source, but make the new snapshot tree follow the source and may
remove extras in the destination. Review the real `rsnapshot -t` output rather
than only the yesh file; the example flags these options and stops before the
explicit confirmation.

The operator therefore asks different questions:

- for `rsync`: “what tree should this destination contain, and what will be
  transferred or deleted?”;
- for `rsnapshot`: “which points in time must remain, which interval transfers
  data now, which old level will be displaced, and is rotation safe to run?”

A practical check for ordinary `rsync -a` is simple: after the source changes,
the changed file appears in the itemized output; a repeat run without new
changes usually reports no transfers. A file that exists only at the
destination remains there: deletion starts only when an option such as
`--delete` is passed explicitly. Review the plan and argument list before the
run instead of inferring the policy from the word “backup”.

💡 yesh does not choose between these tools. It makes the boundary around each
  one reviewable: checks, a temporary tree, and publication for one `rsync`
  result; `configtest`, test mode, rotation review, and explicit confirmation
  for `rsnapshot`. Transfer and rotation remain the external tool's
  responsibility.

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
sync_first = environment YESH_RSNAPSHOT_SYNC_FIRST
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
    if findmnt --source $mount_source --target $root:
        if free_space $root at_least 20MiB:
            rsnapshot -c $config configtest
            if $sync_first matches regex "1":
                rsnapshot -c $config -t sync | tee $report
                rsnapshot -c $config -t $level | tee -a $report
            else:
                rsnapshot -c $config -t $level | tee $report
            if exists $confirm:
                if $sync_first matches regex "1":
                    rsnapshot -c $config sync
                rsnapshot -c $config $level
                if exists $published:
                    if exists $lock:
                        echo "rsnapshot lockfile remains after successful interval"
                        exit 6
                    else:
                        echo "rsnapshot interval completed"
```

The full file also reports an unmounted destination and low free space.
`YESH_RSNAPSHOT_SYNC_FIRST` must be copied from the configuration.
When it is `1`, the plan and the real run contain `sync` followed by the
selected rotation interval; when it is `0`, the selected interval performs the
transfer and rotation. The order is readable from top to bottom: check the
configuration, exact mount, and free space; run `configtest`; show and save the
test-mode report through `tee`; flag known deletion-related rsync options;
require a pre-created confirmation file; apply the correct operation; then
check for the expected `daily.0` and a released lock file.

💡 This is what yesh simplifies: the preflight order, the boundary between test
mode and the changing command, the exact visible `rsnapshot` arguments, and
the failure path are ordinary blocks instead of Bash-specific control-flow
machinery. In `--plan`, yesh shows both branches without running `findmnt` or
`rsnapshot` and without opening redirected files.

The Bash idea would be:

```bash
rsnapshot -c "$config" configtest || exit 1
if [ "$sync_first" = 1 ]; then
    rsnapshot -c "$config" -t sync | tee "$report" || exit 1
    rsnapshot -c "$config" -t "$level" | tee -a "$report" || exit 1
else
    rsnapshot -c "$config" -t "$level" | tee "$report" || exit 1
fi
test -e "$confirm" || exit 4
if [ "$sync_first" = 1 ]; then
    rsnapshot -c "$config" sync || exit 1
fi
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

## SSH keys for a remote source

### What problem does yesh solve?

Without explicit SSH setup, a script can hang on a password prompt or accept an
unverified host key. yesh keeps connection options visible and stops instead of
using a hidden prompt; OpenSSH remains responsible for keys, host trust, and
permissions.

Remote `rsync`, `rsnapshot`, `ssh`, and `scp` do not receive a password from a
yesh script. OpenSSH takes a key from its normal configuration, an agent, or an
explicit file. The private key stays on the machine running yesh; only the
public key is installed on the remote machine.

For a separate test account, setup looks like this:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/yesh_test_ed25519
ssh-copy-id -i ~/.ssh/yesh_test_ed25519.pub backup@example.test
```

Before automation, verify the host-key fingerprint through a trusted channel and
save the host key in `~/.ssh/known_hosts`. `ssh-keyscan` can retrieve a key, but
it does not prove that the host is genuine by itself; compare its result with a
fingerprint obtained from the host owner or another trusted source.

Test non-interactive authentication:

```bash
ssh -o BatchMode=yes \
    -o ConnectTimeout=5 \
    -o StrictHostKeyChecking=yes \
    backup@example.test true
```

The yesh script keeps these choices visible:

```yesh
host = environment YESH_SSH_HOST
ssh_options = [-o, BatchMode=yes, -o, ConnectTimeout=5, -o, StrictHostKeyChecking=yes]

if ssh $ssh_options $host test -d /srv/data:
    echo "remote source is ready"
else:
    echo "remote source is unavailable"
    exit 2
```

For `rsync`, pass the same options through `-e` or an ordinary `~/.ssh/config`
entry. Do not enable `StrictHostKeyChecking=no`, store a private key or password
in a `.yesh` file, or use `sshpass`. `BatchMode=yes` makes a missing key an
explicit failure instead of a hanging password prompt.

The rule is: yesh checks the order and boundaries of the remote call; OpenSSH
handles the key, host trust, and user permissions.

Typical failures here are not yesh failures: a wrong user, an uninstalled public
key, a missing `known_hosts` entry, a changed fingerprint, a forbidden command,
or missing `rsync` on the remote host. Test the same SSH call separately, inspect
the yesh plan, and only then run the changing step.

## 13b. Measuring a systemd service over a fixed period

New in `0.2.0`: read-only service resource snapshots and finite measurements with
bounded JSONL history. This chapter describes the current Linux/systemd backend.

On a Linux host using the unified cgroup v2 hierarchy, systemd manages the
hierarchy mounted at `/sys/fs/cgroup`. It creates or assigns a separate cgroup
for each service unit; for example, `your-service.service` normally appears at
`/sys/fs/cgroup/system.slice/your-service.service/`. That cgroup is the root of
the service tree that yesh observes. A service may have additional child
cgroups, but it needs `Delegate=yes` to manage those children itself.

### What problem does yesh solve?

Without a joined snapshot, an administrator must correlate the service, cgroup,
PIDs, and counters by hand and may combine two invocations. yesh collects them
into a finite measurement, saves the history, and stops comparison when the
service identity changes.

The question is not only “what is wrong with the service right now?” We also
want to know what happened during the last hour or day.

The idea is simple:

```text
measure → save snapshots → summarize → see the change
```

For example, we can see whether memory grew, how much CPU time and disk I/O were
used, whether OOM (Out Of Memory) events occurred, and whether the strange
behavior began after a restart. One snapshot answers only “what is happening
now.” `measure` takes several snapshots over a finite interval and saves them
as one result.

Here is a complete example:

```yesh
# The directory contains measurements.jsonl and its lock file. summarize reads
# the saved history from there; /tmp is used only for this short exercise.
state_dir = "/tmp/yesh-nginx-resources"

measure service nginx.service for 30s every 5s:
    save to $state_dir
    keep history for 24h
summarize service nginx.service from $state_dir since 1h
```

The `measure` line contains the settings for the measurement:

- `service` says that the subject is a service managed by systemd;
- `nginx.service` is the exact service name;
- `for 30s` limits the whole measurement to thirty seconds;
- `every 5s` requests a snapshot every five seconds.

There is no separate `unit` keyword: the service name simply follows `service`
and, when present, `user` or `system`. `save to` selects the history path. In
this example both commands use the same `$state_dir`: the first writes there
and the second reads there. A path ending in `.jsonl` names the journal file
directly; any other path names a directory containing `measurements.jsonl`.
After a successful write, the `keep history for 24h`
directive removes results older than one day from that journal.

The history is stored in `measurements.jsonl`. This is not one large JSON
document and not a command log: each completed `measure` run is one independent
line, while that run's samples are stored inside the record. The record is
appended only after collection finishes. If the process stops in the middle of
the final line, older records remain readable; if a complete line is corrupted,
yesh stops with an error instead of silently skipping it. JSONL is therefore
appropriate for bounded local history, but it is not a database or a resume
mechanism.

When `save to` is omitted, the default journal path is
`~/.local/yesh/<unit>.jsonl`. yesh creates only the missing user-owned parent
directory with mode `0700` and does not require or invoke `sudo`; it does not
create or repair a privileged parent. Reading a system service or its journal may
still require permissions granted by the host. An explicit state path must be
writable by the current user, and yesh does not change its existing permissions.

The colon after the first line means that the following indented lines are
settings for that measurement. This is the extended form. If you do not need to
set the directory or retention window explicitly, use the short form:

```yesh
measure service nginx.service for 30s every 5s
```

The short form still saves the result in the default directory, but it does not
set a retention policy. For long-term use, specify how long to keep the history.

The measurement is finite: the first snapshot is immediate, later snapshots
use the requested interval, and the record is appended only after collection
finishes. `summarize` only reads the history; it does not start the service or
take new snapshots. Start by inspecting the plan:

```text
yesh --plan service-measurement.yesh
```

💡 This is what yesh simplifies: one readable check keeps the service, the
observation period, the saved history, the resource changes, and the relevant
journal window together. The Linux commands remain visible, but their scope,
order, and limits no longer have to be reconstructed from a collection of
separate shell commands.

Do not confuse a partial snapshot with an incomplete measurement. A snapshot can
be partial because an optional field such as block I/O is unavailable; the
saved interval can still be `complete=true` when its deadline finished and the
service identity stayed stable. A missing cgroup or a process disappearing
during collection makes the saved interval incomplete.

The next run can read the old history. `summarize ... since 1h` selects records
from the last hour, compares the beginning and end of each measurement, combines
them into one report, and shows changes in memory, RSS, CPU, OOM, and I/O. It
does not continue an interrupted measurement and does not combine records from
different service invocations. The human report marks memory and RSS direction
with `↑`, `↓`, `=`, or `~`; the navigator also shows the change between adjacent
saved records. To continue, start a new explicit `measure`.

Here RSS means resident memory: the amount of a process's memory currently held
in RAM. Shared pages may be counted for more than one process, and swapped-out
pages are not part of RSS. The kernel's RSS value is itself an estimate, so this
is a per-process observation, not the service's exact unique memory cost.

The result has important boundaries:

- it observes the selected service's cgroup and, where applicable, its nested
  cgroup subtree at a point in time; it does not prove the cause of growth or
  failure;
- `memory.current` is the memory value reported for the observed cgroup scope,
  while RSS describes processes; a sum of RSS values must not be presented as
  `memory.current`;
- memory and RSS are gauges, while CPU and I/O are counters that need deltas
  between samples;
- an unavailable metric is reported as `unavailable`, not as zero;
- history is a bounded JSON Lines journal: an incomplete final line is ignored,
  while a malformed complete record is an error.

An ordinary systemd unit does not need to be changed: systemd already places its
processes in the unit's cgroup. `Delegate=yes` is needed only when the service
creates nested cgroups and places its own processes in them. `CPUAccounting=`,
`MemoryAccounting=`, and `IOAccounting=` can make individual counters available,
but availability also depends on the kernel, systemd, and the active policy.
Therefore yesh reports an unavailable counter as `unavailable`, not as zero.

Each record is tied to the systemd manager, exact unit, cgroup path, and the
unit invocation identity (`InvocationID` when available). A service restart must
not silently merge two invocations into one history. The report describes
measured evidence; it does not invent a causal explanation.

In Bash, the same task usually needs a manual loop, timer, temporary file, and
separate cleanup:

```bash
for i in 1 2 3 4 5; do
    systemctl show nginx.service
    sleep 2
done
```

This loop only prints the state several times. History, comparison, and cleanup
would have to be added separately.

The Bash loop alone does not say where to keep the result, how long to keep it,
or how to avoid mixing different service invocations. These boundaries are
explicit in yesh. Linux, an available systemd manager, and readable cgroup v2
are required.

## 13c. Rotating a service's text logs

### What problem does yesh solve?

A single script can accidentally manage both journald and `logrotate`, duplicate
deletion, or restart a service without checking the result. yesh keeps ownership
separate, connects the dry run to the changing invocation, and checks the service
afterward; `logrotate` still performs rotation and compression.

Consider a realistic maintenance task: nginx has two independent message
streams. Service output is read from `systemd-journald`, while
`/var/log/nginx/*.log` is managed by `logrotate`. They are not one shared log,
so one command must not silently manage both sources.

yesh does not delete old files or guess who owns a path. Its role is to make the
order visible: check the service and disk space, preview the active `logrotate`
policy, require confirmation before changing anything, apply that same policy,
and check the result. Policy, state, compression, removal, and `postrotate`
remain the responsibility of `logrotate`.

The complete script is
[`examples/nginx-logrotate-maintenance.yesh`](../examples/nginx-logrotate-maintenance.yesh).
Its central order is:

```yesh
with lock $lock:
    service status nginx.service
    df -P -h -- /var/log
    $binary -d -v -s $state $config

    if exists $confirm:
        $binary -s $state $config
    else:
        echo "nginx logrotate plan reviewed; confirmation file is missing"
        exit 5

    df -P -h -- /var/log
    service status nginx.service
    journal nginx.service since 5m lines 40
```

`-d -v` is `logrotate`'s preview: it reads the policy and state file but does
not rotate logs. Only an operator-created confirmation file allows the changing
call. The file's contents are not a secret and are not read. Without it, the
script stops before changing logs.

The final `journal` query is observation only; it does not rotate or vacuum
journald. Journal cleanup is a separate operation using `journalctl --rotate`
and `--vacuum-*`, with its own plan and confirmation. The scenario applies only
when `$config` is the active policy for these files; yesh does not prove that
ownership automatically.

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

The word “check” covers several different mechanisms here:

| Mechanism | What it establishes |
| --- | --- |
| Parsing | Correct syntax and supported constructs |
| `--plan` | Intended actions and arguments without execution; both conditional branches are shown as possibilities |
| Check before an action | System state at that moment, such as an existing path or the expected filesystem |
| Result after an action | An observable result, such as service state or a published file |
| Test on a temporary stand | Behavior of a particular script in one tested environment |

None of these replaces the others: successful parsing does not check the host,
the plan does not run commands, and a fixture test does not promise identical
behavior on another system.

Before you run it for real, check that:

- required environment variables are set and non-empty;
- every value is expanded explicitly with `$name`;
- each external command receives the arguments you expect;
- commands that change data use temporary paths until publication;
- an external failure cannot produce a false success message;
- `--plan` shows the possible action structure and both conditional branches;
  it does not claim which branch the real run will select.

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
- [pipeline safety](underthehood/pipeline-safety.md) for pipelines, redirections, and process
  cancellation;
- the [language manual](../man/yesh-language.7) for the complete syntax.

For a selected-snapshot restore workflow, see
[`rsync-selected-restore.yesh`](../examples/rsync-selected-restore.yesh) and its
test fixture [`scripts/yesh-selected-restore-rwe.sh`](../scripts/yesh-selected-restore-rwe.sh).
