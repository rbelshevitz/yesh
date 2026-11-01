# Service resource observation findings

Verification date: 1 November 2026.

Test host: Debian 12, Linux kernel `6.1.0-49-amd64`, systemd `252`, unified
cgroup v2 hierarchy. A cgroup is a
kernel control-group node. Each process belongs to one cgroup in a hierarchy;
cgroups may be nested, and a parent may account for its descendants.

## What was checked

The goal was to establish which evidence yesh can safely collect from one
Linux host:

- the cgroup path and the service process set;
- current memory, resident process memory (RSS), PSI, CPU, and OOM counters;
- whether block I/O accounting is available through `io.stat`;
- a finite multi-point measurement with JSON state storage;
- the difference between an unavailable metric and a zero value.

The check did not change nginx, its limits, or persistent configuration. The
only temporary policy change was enabling `IOAccounting` to test the I/O
controller. The temporary load service removed its own directory afterwards.

## Primary sources and resulting decisions

| Primary source | Rule taken from it | yesh consequence | Evidence |
|---|---|---|---|
| [Linux cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html) | A process belongs to one cgroup; `cgroup.procs` reports process membership with special rules for threaded mode; controller files can be unavailable; several counters are hierarchical | Treat `ControlGroup=` as a tree root, walk nested `cgroup.procs`, preserve `unavailable`, and read aggregate cgroup counters as the primary resource evidence | `readServiceCgroup`, Linux resource tests, nested-cgroup fixture |
| [systemd resource control](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html) | `Delegate=` transfers ownership of the subtree below the unit; the unit exposes its `ControlGroup`, and resource accounting depends on enabled controllers | Do not rearrange or configure cgroups; observe the delegated subtree and never enable `io` automatically | `service resources` output, Debian user/root fixtures |
| [systemd.exec](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html) | `InvocationID` identifies one runtime cycle of a unit | Do not combine measurements across changed invocations | identity-change tests and summary validation |
| [Linux `/proc` documentation](https://www.kernel.org/doc/html/latest/filesystems/proc.html) | RSS is asynchronous and not a precise physical-memory snapshot | Report RSS as an estimate and keep it separate from `memory.current` | process snapshot tests and resource report wording |
| [Linux taskstats](https://www.kernel.org/doc/html/latest/accounting/taskstats.html) | Userspace aggregation can be inaccurate because it is not atomic | Treat the process table as a bounded observation; recheck cgroup membership and mark disappearance | process-membership tests and `processes.incomplete` |
| [systemd-cgtop](https://www.freedesktop.org/software/systemd/man/253/systemd-cgtop.html) | Resource accounting is incomplete when the relevant controller is not enabled | Keep `complete` for interval/identity completeness, while individual metrics remain `unavailable` | Debian fixture: complete memory measurement with unavailable I/O |
| [`go-systemd` D-Bus API](https://pkg.go.dev/github.com/coreos/go-systemd/v22/dbus) and [containerd/cgroups](https://github.com/containerd/cgroups) | Libraries exist for direct systemd API access and cgroup management | Add no dependency: the current read-only boundary is clearer with Go standard library, `/proc`, cgroup files, and visible `systemctl` | `go.mod`, build and dependency review |

These sources establish the interpretation and limits; they do not prove that a
particular host exposes every controller or that a reported resource caused an
application failure. Host-specific availability still requires the Linux
fixture.

## nginx's initial state

For `nginx.service`, systemd reported:

```text
ControlGroup=/system.slice/nginx.service
InvocationID=1cf717120ee8429299078a316af8c2bf
ActiveState=active
MainPID=497
```

The service cgroup exposed only the `memory` and `pids` controllers. `io.stat`
and `cpu.stat.local` were absent, while memory, process, RSS, PSI,
`memory.events`, and `memory.events.local` were available.

A yesh snapshot reported:

```text
process_count=3
rss_bytes=9170944
memory_current=6721536
memory_max=max
oom=0
oom_kill=0
io_stat_available=unavailable
observation=partial
```

`partial` is correct here. An absent controller does not mean zero activity; it
means that this evidence was unavailable.

The process RSS values come from `/proc/<pid>/stat`. They are resident-page
estimates, not an exact physical-memory bill: shared pages may be counted for
more than one process, and the kernel documents the value as inaccurate.

## Enabling I/O accounting

The check used
[`enable-nginx-io-accounting.sh`](../../scripts/enable-nginx-io-accounting.sh):

```bash
sudo /tmp/enable-nginx-io-accounting.sh
```

The script runs:

```bash
systemctl set-property --runtime nginx.service IOAccounting=yes
```

Afterwards nginx exposed `io memory pids`, and `io_stat_available=yes`. Its
`io.stat` was still empty: browser traffic created network requests but did not
prove block reads from disk. Page-cache hits and requests that did not reach
this nginx explain why network traffic need not increase `io.stat`.

## Positive block-I/O fixture

The positive check used the temporary service started by
[`start-io-mock-service.sh`](../../scripts/start-io-mock-service.sh):

```bash
sudo /tmp/start-io-mock-service.sh
```

The service:

- runs through `systemd-run`;
- receives `IOAccounting=yes`;
- has `RuntimeMaxSec=90s`;
- repeatedly writes and reads a temporary file;
- removes the temporary directory after completion;
- does not change nginx or persistent unit files.

For `yesh-io-mock.service`, the kernel reported device `8:0`:

```text
rbytes=536961024
wbytes=662700032
rios=515
wios=554
dbytes=0
dios=0
```

One ten-second yesh measurement with a two-second interval saved six complete
points:

```text
SERVICE_MEASURE ... samples=6 complete=true
memory: 70.6 MiB -> 133 MiB
OOM kills: 0
```

After its time limit the service exited with `Result=success`, code `0`, and
the temporary directory was removed.

## Conclusions for yesh

1. `io` is optional. yesh must not enable it automatically: that changes the
   systemd and cgroup policy for the whole branch.
2. Read `io.stat` as counters per `major:minor` device, not as one service-wide
   number. The kernel does not promise row order.
3. Missing `io.stat`, `cpu.stat.local`, or individual fields must be reported as
   `unavailable`, never replaced with zero.
4. PSI shows pressure, but by itself does not prove which process caused a
   delay.
5. Browser traffic is suitable for checking the network, not for proving block
   I/O. That requires a controlled fixture with real reads and writes.
6. One snapshot describes a state; a period requires a finite measurement with
   several points and preserved source data.
7. A cgroup snapshot shows the node, its direct process members, and any nested
   cgroups covered by the observation; it does not prove application ownership
   or container identity.

## What a systemd service cgroup means

`ControlGroup=` is the cgroup path of the unit's control-group root. It is a
boundary for the unit, not a process list frozen at one exact moment and not
necessarily the only directory that contains its processes.

Without delegation, systemd owns the control-group subtree and keeps process
placement and resource-control policy under the manager's control. With
`Delegate=yes`, systemd gives the unit ownership of the subtree below the
unit's cgroup. The unit may then create nested cgroups and move its own
processes into them; systemd deliberately refrains from rearranging that
delegated subtree. `DelegateSubgroup=` is a related systemd option that places
the service's main process in a named child subgroup.

Therefore yesh treats `ControlGroup=` as the root of an observation tree,
walks its `cgroup.procs` files recursively, and records whether the process
list was incomplete. It does not assume that the root's `cgroup.procs` is the
whole service. The kernel can change membership while the files and `/proc`
are being read, so the result is a short observation assembled over a small
time window, not one perfectly consistent picture. A process that has visibly
moved outside the tree is excluded; a process moved into the tree after the
walk may still be missed.

This interpretation follows systemd's
[`Delegate=` documentation](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html)
and the kernel's
[cgroup v2 hierarchy rules](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html).

## Nested-cgroup probe on the Debian host

A temporary user service with `Delegate=yes` was used to create a child cgroup
on the Debian test host. The delegated branch exposed `cpu memory pids`, and
the test enabled `cpu` and `memory` for the child after moving the supervising
process out of the parent. The parent and child CPU counters both advanced; the
parent delta also included work outside the child, which is consistent with
hierarchical `cpu.stat` accounting.

The same user service did not receive the `io` controller. Its
`cgroup.controllers` remained `cpu memory pids`, and `io.stat` was unavailable in
both the parent and child. Adding `IOAccounting=yes` to the transient user unit
did not change that. This is a permission and delegation boundary, not evidence
of zero I/O.

The repeatable root fixture is
[`scripts/yesh-nested-cgroup-rwe.sh`](../../scripts/yesh-nested-cgroup-rwe.sh).
On the Debian 12 host it enabled `cpu io memory` for a child and performed a
32 MiB direct write from that child. The parent and child both reported
`wbytes=33554432` and `wios=32`, while the parent CPU delta was larger than the
child delta. This confirms on the tested kernel that the parent `io.stat`, like
the tested CPU and memory counters, covers the child cgroup. It is evidence for
this kernel and setup, not a universal claim about every cgroup configuration.

The source semantics were checked against the Linux
[cgroup v2 documentation](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
and systemd's
[resource-control documentation](https://www.freedesktop.org/software/systemd/man/252/systemd.resource-control.html).

## State storage

The first prototype stored one completed measurement in a separate `*.json`
file. The current format is one bounded `measurements.jsonl` JSON Lines journal
per state directory:

- one completed JSON record per line;
- one journal per service, rather than one file and inode per measurement;
- appends under a lock;
- bounded record size;
- an incomplete final line does not make the whole journal unreadable;
- cleanup uses a temporary file and atomic rename;
- old separate `*.json` files may be read during migration.

This limits inode use: two appended measurements remain one
`measurements.jsonl` plus the lock file. Cleanup temporarily creates one extra
file, which is removed after the atomic replacement. Future policy must limit
both journal size in bytes and the number of service journals, not only record
count.

The practical history window is at least 24 hours: `summarize ... since 24h`
must not lose the beginning of the period through cleanup. Retention therefore
needs to keep 24 hours plus a small margin; cleanup beyond that window should
remain an explicit action.

`summarize` compares `rbytes`, `wbytes`, `rios`, `wios`, `dbytes`, and `dios`
per device. A device-set change, an unavailable point, or a counter reset is an
incomplete comparison; devices are not silently merged into one unnamed value.

## Keeping the JSON Lines journal safe

JSON Lines is not one large JSON document and must not have outer `[` and `]`.
Each line is an independent JSON object. A record is complete only after its
newline (`LF`) has been written.

The write rules are:

1. Build and validate one bounded JSON object completely in memory.
2. Add one `LF` and append it with `O_APPEND`.
3. Use a yesh lock for multiple writers; `O_APPEND` alone is not an ownership
   policy.
4. Handle short writes, then call `fsync` on the file. To make the name survive
   a power loss, call `fsync` on the directory separately.
5. Do not publish a record before its `LF`. If a process stops after partial
   JSON or before `LF`, reading stops at the incomplete tail and preserves
   earlier records.
6. A complete but syntactically invalid line is a journal error, not data to
   silently skip. This distinguishes corruption from a normal incomplete tail.

On an ordinary local filesystem, `O_APPEND` combines selecting the end of the
file with the write operation, but it does not guarantee survival after power
loss. NFS has separate `O_APPEND` limitations, so state should be kept on a
local filesystem or the weaker boundary must be explicit.

This follows the general model of streaming JSON sequences: independent
records are processed one at a time, and an incomplete final record must not
make the whole stream unreadable. RFC 7464 uses an additional `RS` delimiter
before each record; yesh currently uses the more familiar JSONL format with
`LF` because ordinary Unix tools read it easily. See
[RFC 7464](https://www.rfc-editor.org/rfc/rfc7464).
