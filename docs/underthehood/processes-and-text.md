# Text Filters, Processes, and cgroups

## The decision on sed and awk

`sed` and `awk` remain external Linux commands. Embedding them would violate
the principle of one clear way to do each thing and require maintaining two
extra grammars,
quoting systems, and state models.

The safer future yesh operations are narrower:

```yesh
search "error" in /var/log --type log
replace "old-host" with "new-host" in config/*.conf
fields 1,3 from /var/log/app.tsv separated by tab
```

`search` is read-only. `replace` builds a plan and does not modify files by
default; applying it requires atomic writing while preserving mode and owner.
Globs are expanded by the runtime, not a shell, and an empty glob is an error.
Binary files, symlinks, and paths outside the operation root require an
explicit decision. Complex logic remains in a separate `.awk` or `.sed` file.

## Process identity

Do not make a direct `pkill pattern` equivalent. Names are limited on Linux and
PIDs can be reused. A process query returns structured records, not stdout:

```yesh
nginx = processes where name is "nginx"
show $nginx fields pid uid state rss start_time cgroup
summarize $nginx memory rss
```

The implemented read-only snapshot matches the exact Linux kernel name and
reports selected fields such as PID, UID, state, RSS, PPID, PGID, SID, name,
and cgroup. `start_time` is the Linux kernel start-time tick count, useful for
distinguishing reused PIDs. RSS is in bytes. The operation sends no signals and changes no
process state. A process that disappears while `/proc` is being read is not
reported as a false result.

Regex filters, arbitrary command-line matching, and signals are separate future
features. A general signal API, `pkill`, `kill -9`, PID-group signalling, and a
protection-bypass mode are deliberately not planned. Own jobs should use their
internal lifecycle; system services should use a checked systemd unit.

## Memory and job ownership

The process model should distinguish:

```text
rss       resident memory in RAM
vsz       virtual address space
pss       proportional share of shared pages
swap      swapped pages, when available
memory    cgroup memory.current for the group
```

RSS is not the complete cost of a process, and VSZ is not physical usage. PSS
is useful for comparing services but requires `smaps_rollup` or an equivalent
source. A single snapshot is enough for a report; dynamic measurements require
two snapshots and an explicit delta. Infinite `top`-style observation is not
allowed inside a script.

## cgroups and systemd

cgroup v2 is a hierarchy in which each process belongs to exactly one group.
`cgroup.procs` lists group PIDs, while controllers apply resource limits
hierarchically. yesh must not write to `/sys/fs/cgroup` without explicit
permissions, delegation, and ownership checks.

For administration, yesh prefers systemd as the owning layer:

```yesh
job = scope "yesh-backup"
run job rsync -a /srv/data /mnt/backup
limit job memory 512MiB
limit job cpu 50%
wait job finished within 1h
```

An implementation may lower this to a `systemd-run` transient scope or service
with explicit `--wait`, `--pipe`, and resource options. Infinite `watch`,
`journalctl -f`, `top`, and `systemd-cgtop` are outside the MVP.

## tmux and screen

Long unattended jobs should prefer a systemd scope or service because it offers
timeouts, cgroups, exit status, and logs. tmux and screen are for interactive
rescue or deploy sessions. Read-only listing and bounded capture are safe
directions; sending arbitrary text is not, because it may execute a command or
terminal escape sequence. Secrets must never be entered through `send-keys` or
`stuff`, and attach/follow is outside the MVP.
