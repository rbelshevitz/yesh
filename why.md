# Why I Built yesh: Linux Automation You Can Explain

> **yesh `v0.2.0` is out.**
>
> This release packages the project as a reviewable Linux/amd64 tool, with guarded backup and restore workflows around `rsync` and `rsnapshot`, staged configuration publication, bounded systemd checks, finite read-only resource observation, locking, reruns, failure handling, and `--plan` coverage. It is early-stage and deliberately keeps the underlying Linux tools visible.

There is a particular kind of sadness in Linux administration.

You open a Bash script that started as six lines. It now checks a mount, copies a release, restarts a service, waits for it to recover, writes a log, and cleans up a temporary directory. The commands themselves are still familiar. The script is not.

You read it once to understand what it does. Then you read it again to work out which quotes preserve an argument, which failure stops the script, whether a pipeline left a child process behind, and whether the old configuration is still intact after a half-completed run. At some point, the operator's real question becomes: “Can I explain this before I run it?”

This is not because Bash is bad. Bash is fast to start, available on almost every Linux system, and excellent when the job is shell work: interactive commands, short glue, and composition of existing Unix tools. I use it and respect it.

Bash can express complete imperative procedures: functions, loops, conditionals, traps, and error handling are all part of the language. Its problem is not the absence of a procedural model. The difficulty is seeing, before execution, how shell syntax, expansions, process rules, and error handling combine into system-level effects. Bash does not know what change you intend to make, and it does not check that the commands match that intention. A syntactically valid command can still point at the wrong path, publish a partial result, or leave the operator without useful evidence that it succeeded.

That distinction is the reason for yesh. Bash describes execution for the shell; yesh makes selected parts of the operational procedure explicit for the person reviewing it: what is checked, what may change, what is published, what happens after a partial failure, and what proves success. It does not guess the meaning of an arbitrary `rsync` command or make an external utility safe by itself.

I built [yesh](https://github.com/rbelshevitz/yesh) for that moment: scripts that stay close to Linux commands while making their arguments, ordering, failure behavior, and planned changes easier to review.

The goal is not to replace Bash, Python, or Ansible. Each tool has a different job. yesh is for the awkward middle: a small administrative script that is too important to hide behind shell tricks, but too close to the Linux command line to justify a larger automation framework.

## The question is not “can it run?”

For operational code, a successful exit status is only one part of the story.

Before running a script, an operator should be able to answer:

- What does it check first?
- What exact arguments reach each external process?
- Which files or services can change?
- What happens if a command fails halfway through?
- What remains unchanged after a failed run?

That is the standard behind yesh. The rest of the design follows from it: keep Linux utilities visible, make the surrounding procedure explicit, and make the failure boundary reviewable before execution.

## The same intent, with a visible boundary

Consider a configuration update:

```bash
set -euo pipefail

if grep -q '^mode=production$' config.next; then
    install -m 0640 -- config.next /etc/myapp.conf.next
    mv -T /etc/myapp.conf.next /etc/myapp.conf
    systemctl restart myapp.service
    systemctl is-active --quiet myapp.service
else
    echo "configuration validation failed" >&2
    exit 2
fi
```

Here is the same operational intent in yesh:

```yesh
if grep -q '^mode=production$' config.next:
    install -m 0640 -- config.next /etc/myapp.conf.next
    publish /etc/myapp.conf.next to /etc/myapp.conf
    service restart myapp.service until active within 10s
else:
    echo "configuration validation failed"
    exit 2
```

The yesh version earns its keep because it reads like a procedure: validate, prepare, publish, restart, and verify. It is not dramatically shorter. It is simply easier to inspect before anyone runs it.

This example assumes that `/etc/myapp.conf` is an absent name or a replaceable symlink managed by the deployment. If it is an ordinary existing file, the deployment needs a different explicit publication layout; `publish` does not silently overwrite or merge it.

The Bash example uses GNU `mv -T`, which is appropriate for the Linux scope of this article. It also leaves the timeout and post-restart check to separate commands; the yesh operation keeps the ten-second deadline and active-state check in one line.

`publish` uses a same-filesystem rename and does not silently fall back to copying. Its destination must be an absent name or a replaceable symlink; it does not merge an existing directory. The service operation has a finite deadline and checks that the exact unit becomes active. The script puts these rules next to the actions instead of leaving a reviewer to reconstruct them.

## Bash is excellent when the job is shell work

Bash remains excellent for interactive work, one-off commands, and the enormous body of existing Unix tooling. yesh is not an argument against using Bash for those jobs.

The boundary appears when the main risk is reviewing the procedure. yesh makes a few deliberate choices to reduce that burden:

```yesh
message = "backup finished"
hosts = [web-1, web-2]

echo $message

for host in $hosts:
    echo $host
```

A string remains one value, a list expands into separate arguments, and a variable is expanded only with the explicit `$name` form. Values are not reparsed as code or split implicitly by whitespace. These rules remove one category of shell reasoning from the review; they do not make external commands safe or guarantee that they will succeed.

Python remains the better choice for substantial application logic, and Ansible remains the better layer for inventory and desired state across many hosts. yesh is for the smaller Linux procedure in between, where the operator needs to see the real `ssh`, `rsync`, or `systemctl` command and the rules around it.

## Commands remain commands

yesh does not turn a line into one shell string and pass it to `sh -c`. Commands are parsed into an argument list and executed directly by the Go runtime.

```yesh
install -m 0640 -- config.next /etc/myapp.conf.next
```

The process receives these separate arguments:

```text
["install", "-m", "0640", "--", "config.next", "/etc/myapp.conf.next"]
```

This keeps argument boundaries visible when values contain spaces, commas, or characters that would have a special meaning to a shell. It also means that yesh does not reinterpret a value as a new command or operator.

Linux tools remain visible and retain their native semantics. `rsync` is still `rsync`; `systemctl` is still `systemctl`; `ssh` is still `ssh`. yesh provides the surrounding checks, ordering, and failure behavior.

## `--plan` makes review practical

The argument boundary is only half of the review problem. Before a script changes state, yesh can show the expanded work without starting processes or opening redirect files.

Bracketed lines are external `argv`. Named lines are yesh operations with their own rules. The output is not a transaction and does not remove the gap between checking and acting. It is a concrete description of what the script would try to do, suitable for review.

Here is a normalized excerpt from a disposable Linux run of `examples/rsync-incremental.yesh`:

```text
PLAN lock "/srv/example-snapshots/.rsync.lock"
PLAN ["rsync" "-a" "--fsync" "--delay-updates" "--" "<environment value>" "<environment value>"]
PLAN publish "/srv/example-snapshots/snapshot.1.tmp" to "/srv/example-snapshots/snapshot.1"
```

The plan makes three things visible before any transfer starts: the lock, the real `rsync` flags, and the point at which the completed tree becomes visible. Environment-sourced values appear as `<environment value>` in plan output, so the review does not print their contents.

A plan is not a second execution or a snapshot of reality. The source can disappear after `exists`, a service can change state before the restart, and an external command can still fail. `--plan` answers “what would this script try to do with these values?” It does not answer “will the machine still have this state when it runs?”

## Failure is part of the design

The plan describes intended work. Execution still needs a clear failure rule. An ordinary command failure stops the script by default. A non-zero result used as a condition can select an explicit `else` branch. A missing executable, startup failure, cancellation, signal, or timeout remains an execution error; yesh does not quietly turn it into a normal branch.

```yesh
source = "/srv/releases/current"

if exists $source:
    echo "release is present"
else:
    echo "release is missing"
    exit 2
```

If any pipeline stage fails, the pipeline fails. On POSIX systems, cancellation terminates the owned process group and waits for child processes instead of only killing the first visible process.

This is not a claim that external programs are safe. `rsync` can still delete data if given destructive arguments. `systemctl` can still interrupt a service. Direct argument execution prevents shell reinterpretation; it does not give an arbitrary executable good intentions.

## The runtime has to earn that promise

The runtime is written in Go, so the hard part is not only parsing commands. It is also making cancellation and cleanup predictable when several processes and goroutines are involved.

The project has more than 200 automated tests and four fuzz targets. POSIX/Linux coverage includes pipeline failure, `SIGPIPE`, process groups, timeouts, cancellation, child cleanup, redirects, and repeated cancellation. Race-detector runs also look for leaked file descriptors and goroutines.

I also ran ten disposable test runs on Debian and Rocky Linux. The release checks exercise the documented Linux workflows under controlled conditions, including real `rsync` and `rsnapshot` transfers, reruns, partial failures, locking, and restore verification. The results support the tested systems and scenarios; they do not prove universal compatibility.

One reproducible test is more informative than the total count. In a disposable Linux fixture, the real external `rsync` runs and a wrapper then returns code 23, simulating a partial-transfer result. The test checks four concrete results: the new snapshot is not published, the previous snapshot is unchanged, the staging directory remains available for diagnosis, and a retry succeeds.

This test protects a specific promise: a partial transfer must not become the backup that a restore job sees, and a failed retry must not destroy the last good snapshot. If the runtime started publishing before `rsync` finished, or treated code 23 as success, this test would fail. It does not prove that every filesystem or network failure behaves the same way; it checks this particular failure case on the tested Linux setup.

That work matters because killing the first process is not the same as stopping a pipeline. Returning an error is not enough if a child or a cleanup goroutine remains behind. The runtime must stop the work it owns and wait for the processes it started.

## What `v0.2.0` adds

The release is deliberately practical rather than expansive. Its less visible addition is finite, read-only resource observation for one systemd service on Linux with cgroup v2. A script can take samples for a fixed period, save a bounded line-by-line JSON history, and summarize memory, CPU, and I/O observations. This is useful during one incident or maintenance window; it is not a monitoring daemon, a cgroup manager, or a promise that unavailable counters are zero.

The release is Linux/amd64-focused. Windows CI covers the portable parser and runtime pieces; Linux integration checks cover process, service, filesystem, and external-tool behavior.

## The boundary

The scope is deliberate. yesh is not an interactive shell, a Bash-compatible replacement, a Python competitor, an Ansible alternative, or a backup engine. It does not grant privileges, make arbitrary external programs safe, or turn a successful command into proof that the system is healthy. It is an early-stage Linux/amd64 release; external programs, privileges, paths, filesystems, kernels, remote hosts, and service behavior remain real operational concerns.

## The case for trying it tomorrow

Take a backup that must not publish a partial result. The important failure is not an exotic parser bug: it is an interrupted transfer that leaves a directory looking complete, or a retry that overwrites the last good copy.

In yesh, the boundary is deliberately visible:

```yesh
source = environment YESH_BACKUP_SOURCE
staging = "/srv/backups/.next"
published = "/srv/backups/current"

rsync -a -- $source $staging
publish $staging to $published
```

The real scenario adds preflight checks, a lock, and a new empty destination with an explicit path for each run. If `rsync` returns a partial-transfer or timeout status, execution stops and `publish` is never reached. The previous published tree remains the one a restore job can see. A rerun gets a new staging path instead of reconstructing cleanup rules from shell traps.

There is a cost. A team adopting yesh takes on another executable, another language to learn, and another runtime to debug. That cost is not justified for a one-line command or a script that Bash already makes clear. It becomes reasonable when the procedure has a real failure boundary—staging and publication, process cancellation, bounded service checks, or restore verification—and the surrounding shell code is harder to review than the underlying Linux commands.

That is the invitation: take one such procedure, read the [syntax tutorial](https://github.com/rbelshevitz/yesh/blob/main/docs/syntax-guide.md), run `--plan`, inspect the expanded arguments, force one failure, and verify what remained unchanged. If the result is not easier to explain to the next operator, use Bash. yesh should earn its place in the toolchain on that test.

[Explore yesh on GitHub](https://github.com/rbelshevitz/yesh) · [Download `v0.2.0`](https://github.com/rbelshevitz/yesh/releases/tag/v0.2.0)
