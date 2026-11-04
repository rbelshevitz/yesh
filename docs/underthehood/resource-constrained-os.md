# Resource-constrained operating systems

This is a project note for a future yesh direction. It records what yesh must
eventually assume when it observes resources on a small or minimal Linux
system. Alpine Linux is the reference case, but the boundary also applies to
containers, rescue systems, embedded installations, and hosts where no service
manager is present.

The initial Alpine/OpenRC adapter layer is implemented: yesh can parse
machine-readable OpenRC state and service records and prepare a finite snapshot
with platform, supervisor, PID, and cgroup identity. Automatic source selection
and execution of the built binary on Alpine are proven for an ordinary service
and a temporary `supervise-daemon` service. This is not continuous monitoring
or full user-service support.

## yesh's role on a small system

On a resource-constrained node, yesh is a finite checking and measurement
script. It answers “what can be proven on this node right now?” and leaves an
explainable result for an operator or an external tool that invoked the script.

yesh does not become another init system, a permanent monitoring agent, or a
desired-state manager. It does not replace OpenRC, `supervisord`, BusyBox,
syslog, or cgroups. Those components remain visible evidence sources; yesh
connects them only for the duration of a finite observation.

The small-system paradigm is:

```text
identify the local platform
→ confirm the manager or supervisor
→ verify the service and PID
→ verify cgroup and available counters
→ read logs if a real source exists
→ save a finite snapshot or report
```

When an interface is missing, yesh reduces detail but never increases
confidence: `unavailable` and `partial` are better than a zero counter or a
guessed state. Writes are limited to an explicitly selected measurement journal
and temporary artifacts owned by the script; yesh does not reconfigure OpenRC,
Supervisor, services, or cgroups.

## Why Alpine is an important reference

As of this note, Alpine's stable release branch is 3.24. Alpine is built
around musl libc and BusyBox, and uses OpenRC rather than systemd as its normal
service manager. BusyBox provides the default `init`, `ash`, and many command
applets. Those applets usually implement a smaller option set than the GNU
utilities with the same names.

The default Alpine shape is therefore materially different from a Debian or
Fedora system with systemd:

```text
BusyBox init (PID 1)
        ↓
      OpenRC
        ↓
  service scripts and processes
        ↓
  optional cgroup v2 hierarchy
```

Alpine's documentation describes OpenRC as the default init system and cgroup
v2 as the default OpenRC mode since Alpine 3.19. The upstream OpenRC guide,
however, describes its general default as using both cgroup versions when they
are available. This difference is a reason to inspect the running host rather
than infer the mode from the distribution name or version. Cgroups can also be
disabled, mounted differently, delegated only partially, or hidden by a
container boundary.

Sources: [Alpine release branches](https://www.alpinelinux.org/releases/),
[Alpine overview](https://wiki.alpinelinux.org/wiki/Alpine_Linux%3AOverview),
[BusyBox in Alpine](https://wiki.alpinelinux.org/wiki/BusyBox), and
[Alpine OpenRC](https://wiki.alpinelinux.org/wiki/OpenRC).

## Choosing an Alpine binary

Alpine does not imply one processor architecture. Check the target node before
copying a binary:

```sh
uname -m
apk --print-arch
```

If the target fixture reports `x86_64`, use:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -o yesh-linux-amd64 ./cmd/yesh
```

Common alternatives change the Go target:

| `uname -m` | `GOARCH` | Example |
| --- | --- | --- |
| `x86_64` | `amd64` | 64-bit x86 example |
| `aarch64` | `arm64` | 64-bit ARM |
| `armv7l` | `arm`, `GOARM=7` | 32-bit ARM |
| `armv6l` | `arm`, `GOARM=6` | older 32-bit ARM |
| `mips`/`mipsel` | `mips`/`mipsle` | only if the Go port is available |

Check supported targets with `go tool dist list`; do not infer the architecture
from the Alpine name alone. `musl` is Alpine's C standard library, not a CPU
architecture. Current yesh has no cgo dependency, so `CGO_ENABLED=0` produces a
portable Go binary without a dynamic glibc or musl dependency. If cgo is added
later, it will need a separate build and a compatible musl toolchain.

Sources: [Go environment variables](https://go.dev/doc/install/source#environment)
and [Alpine's musl notes](https://wiki.alpinelinux.org/wiki/Musl).

## Identify the OS separately from the service manager

yesh should identify the local operating system by reading `/etc/os-release`,
with `/usr/lib/os-release` as the fallback, and checking the machine-readable
field `ID=alpine`. `PRETTY_NAME` is for display; it is not a reliable control
value. `ID_LIKE` may help classify an otherwise unknown derivative, but it must
not relabel that system as Alpine.

This answers only which operating system is present. It must not by itself
select the resource or service implementation. Alpine normally uses OpenRC,
but OpenRC is also used outside Alpine, and an Alpine image can run without a
usable OpenRC state or inside a container with a different PID 1.

The service-manager probe must therefore be separate and operational:

```text
local os-release → ID=alpine (identity only)
        ↓
confirm systemd or OpenRC interfaces and state
        ↓
confirm the selected service, PID, and cgroup
```

For OpenRC, the adapter may inspect the documented `openrc`/`rc-*` interface
and its local state, then still has to verify the service process and cgroup.
The presence of an executable alone is not enough. If the probes disagree or
no manager is confirmed, yesh must report an unknown/unavailable service
manager and may offer only the resource view that the kernel interfaces prove.
It must not silently present an Alpine-specific or systemd-shaped form.

In a container, use the local `/etc/os-release` for the environment in which
yesh runs. `/run/host/os-release`, when supplied by a runtime, describes the
host and must not be mistaken for the container's own operating system.

Source: [the os-release specification](https://www.freedesktop.org/software/systemd/man/latest/os-release.html).

## Future: one service view

`supervisord` and container observation are not implemented yet. Current support
is limited to systemd and OpenRC; the material below is a note for future work,
not a description of a ready interface.

The user-facing object should remain a service. The user should not have to
choose between systemd, OpenRC, `supervise-daemon`, and `supervisord` before
asking a basic question about a running program.

The adapter may choose its source after probing the local system and the exact
service name. It must then preserve the evidence layers instead of flattening
them into one guessed state:

```text
service name
→ confirmed manager or supervisor
→ reported service/process state
→ verified PID and process start time
→ verified cgroup membership
→ finite resource snapshot
```

The first read-only sources are:

| Source | What yesh may read | Important boundary |
| --- | --- | --- |
| systemd | `systemctl show` properties such as `MainPID`, state, cgroup, and invocation identity | runtime unit state is not the same as a durable configuration file |
| OpenRC | `rc-status` machine-readable `ini` output, service state, `/proc`, and cgroup | OpenRC state and a supervised child process are separate facts |
| `supervisord` | its XML-RPC process information and configured log paths, plus `/proc` and cgroup | a supervisor-reported `RUNNING` state does not prove resource ownership |

Human-oriented output such as coloured status lines must not be the primary
parser input when a documented machine-readable interface is available. A
missing interface, permission failure, or unsupported field remains
`unavailable`; yesh must not replace it with zero or infer it from a process
name.

`supervise-daemon` is an OpenRC supervision detail, not a separate user-facing
service kind. If it respawns a child, the observation must retain the new PID
and start time and mark the process epoch as changed. A `supervisord` program
may similarly expose a process state, PID, start/stop times, exit status, and
log paths; those facts are useful context, while Linux `/proc` and cgroup
checks remain the evidence for the actual process and resource view.

This is a common observation contract, not a promise that every manager
supports every action. The first common operations should be read-only status,
resource snapshot, and finite measurement. Start, stop, restart, log rotation,
and manager-specific controls remain outside the common contract until each
has its own positive, negative, and `--plan` proof.

Sources: [OpenRC `rc-status` manual](https://github.com/OpenRC/openrc/blob/master/man/rc-status.8),
[Supervisor XML-RPC API](https://www.supervisord.org/api.html), and
[`systemctl` documentation](https://freedesktop.org/software/systemd/man/latest/systemctl.html).

## Observing `supervisord` in a container

`supervisord` is a separate process supervisor, not another OpenRC mode. In a
container it may be PID 1 and launch several foreground processes, but PID 1
alone does not prove that a particular program is managed by Supervisor.

The first Supervisor request should check the XML-RPC API version. The adapter
can then request one exact `getProcessInfo(name)` record or a bounded list with
`getAllProcessInfo()`. The record provides state, PID, start and stop times,
exit status, and log paths. `RUNNING` remains a supervisor claim: yesh must
also verify the PID through `/proc`, its start time, and cgroup membership.

The XML-RPC endpoint may be a Unix socket configured by `[unix_http_server]` or
HTTP/TCP configured by `[inet_http_server]`. The TCP interface must not be
enabled or discovered by scanning addresses: it may require a username and
password, has no encryption, and is intended only for a trusted network. If
the interface is disabled, inaccessible, or its socket path is unknown, the
report must say `supervisor=unknown` rather than infer state from a process
name.

Supervisor's `group:name` is a logical process group, not a Linux cgroup. A
group may contain several programs, so yesh must not replace it with one PID or
one memory number. A snapshot must list verified PIDs, their start times, and
the common observable cgroup boundary when that boundary is actually proven.
If the processes occupy different cgroups, the group's complete resource view
is unavailable or incomplete.

Container execution also matters because Supervisor expects child programs to
stay in the foreground. A program that daemonizes itself and creates a new
daemon may no longer have the expected supervisor relationship. In that case,
the XML-RPC state, parent PID, and actual cgroup must remain separate evidence
layers.

Sources: [Supervisor introduction](https://docs.supervisord.org/introduction.html),
[Unix-socket and TCP configuration](https://docs.supervisord.org/configuration.html),
[Supervisor subprocesses](https://www.dist.supervisord.org/subprocess.html), and
[the XML-RPC API](https://www.supervisord.org/api.html).

## There is no mandatory journal layer

Alpine normally uses BusyBox `syslogd` and may write to files such as
`/var/log/messages`; `logread` may be the appropriate reader. An installation
can instead use `rsyslog`, `syslog-ng`, `logbookd`, or another logging setup.
`journald` is not a prerequisite and must not be silently assumed.

This means that a resource observation may have no journal context. That is a
valid result, not a failed resource measurement. The report should distinguish:

- resource data that was read successfully;
- service state supplied by an init-system adapter;
- log data supplied by a logging adapter;
- data that was unavailable on this host.

Source: [Alpine syslog documentation](https://wiki.alpinelinux.org/wiki/Syslog).

## How OpenRC identifies a service

OpenRC is not a persistent manager daemon in the systemd sense. Its commands
apply a runlevel or service action and then exit; service scripts and optional
supervisors keep the actual processes running. A yesh adapter therefore cannot
assume a manager API, a D-Bus object, or a stable manager-issued invocation
identifier.

OpenRC service scripts can track a daemon through a PID file, a process name,
`start-stop-daemon`, or `supervise-daemon`. OpenRC can also place service
processes in cgroups and apply per-service cgroup settings. The exact service
identity is consequently not equivalent to a systemd unit identity:

- there is no general `InvocationID` equivalent to use as a portable contract;
- a PID file may be stale or point at a process that has changed;
- a service may fork, supervise children, or create additional descendants;
- the cgroup layout and its controller set depend on host configuration;
- the service's logs may be files or syslog records rather than a journal.

When `supervise-daemon` is used, the supervisor process and the daemon it
manages are separate identities. A resource observation must follow the
verified service cgroup and its descendants, not report only the supervisor's
PID. Output may also be routed through OpenRC logging options, so the process
measurement and log source must remain separate. `supervise-daemon` expects the
managed daemon to stay in the foreground, can respawn it after an unexpected
exit, and can run periodic health checks. A measurement interval can therefore
cross more than one process lifetime and must record a process restart instead
of silently combining the lifetimes.

The child process exit code is a separate observation. `255` is not a special
yesh code and does not prove that `supervise-daemon` itself exited: it may be a
normal code returned by the managed program, such as `openrc-user`. If the
child PID disappears, service state, respawn count, and epoch must be checked
independently. The yesh adapter currently exposes the epoch and incomplete
measurement, but does not publish the child exit code as a separate JSONL field.

For an OpenRC adapter, yesh must discover and verify the process and cgroup
identity. It must not infer a cgroup path from a service name alone. The Linux
kernel documents that `cgroup.procs` is not ordered and can expose races with
process movement or PID reuse.

Sources: [OpenRC service-script guide](https://github.com/OpenRC/openrc/blob/master/service-script-guide.md),
[OpenRC user guide](https://github.com/OpenRC/openrc/blob/master/user-guide.md),
and [Linux cgroup v2 documentation](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html).

## Resource observation must be Linux-level first

The resource collector should not require systemd. Its primary read-only
sources are:

```text
/proc                         process identity, status, RSS, start time
/proc/<pid>/cgroup             process-to-cgroup membership
/sys/fs/cgroup                 cgroup v2 counters and controller availability
/proc/pressure                 host-wide PSI, when readable
```

The target adapters then add context:

| Target | Strongest available identity | Optional context |
| --- | --- | --- |
| systemd service | unit, `InvocationID`, cgroup | journal and manager state |
| OpenRC service | verified PID/cgroup and service state | syslog or service log |
| arbitrary cgroup | cgroup path and subtree | container/runtime metadata |
| process tree | PID plus process start time and membership checks | command line and parentage |

The common measurement result must keep the same distinction as on a systemd
host: a missing controller or permission is `unavailable`, not zero. A process
that disappears during collection makes the relevant observation incomplete.

## Consequences for yesh

The current `service resources` and `measure service` forms are initial systemd
and OpenRC adapters, not the definition of the resource model. Without an
explicit profile, yesh confirms each available source separately and refuses
to guess when probes conflict. The future model should
remain:

```text
target → confirmed identity → finite interval
→ measurements → completeness → explainable result
```

This has several practical consequences:

1. A systemd manager is not a prerequisite for the core collector.
2. `journal` is an optional context layer, not part of resource validity.
3. The collector must read kernel interfaces directly or use a visible,
   narrowly defined adapter; it must not depend on GNU command flags being
   present in BusyBox.
4. The navigator must explain which layers were available on the selected
   host instead of presenting a systemd-shaped form for every Linux system.
5. A rootless or containerized run may see only a delegated cgroup subtree and
   a restricted process view. The report must preserve that limitation.

yesh is still not a resource controller, a daemon, or a replacement for a
long-running monitoring stack. It performs finite, explainable observations.

## Required Alpine validation stand

Before adding an OpenRC-specific language operation, validate the lower-level
collector on an Alpine VM or an explicitly configured test image:

1. a positive OpenRC-managed service with readable `/proc` and cgroup v2;
2. a service with no journal, but with readable syslog or service output;
3. an unavailable controller, reported as `unavailable`;
4. a process that exits or restarts during the interval;
5. a cgroup mount or delegation that is unavailable;
6. a container view that sees only part of the host hierarchy;
7. `--plan` with no reads, writes, service changes, or hidden discovery.

The VM is the primary proof for init and cgroup behavior. A container is a
separate proof of a restricted view, not a substitute for the VM. A passing
Alpine test proves the selected Alpine/kernel/container setup only; it does not
prove behavior on every OpenRC distribution or every Linux kernel.

The current Alpine VM result is deliberately narrow: `ID=alpine` and OpenRC
were confirmed; `rc-status -C -f ini -a` reported services from all runlevels,
including stopped ones; and a
separate `lighttpd` run confirmed a clean service boundary: one process in
`/openrc.lighttpd`, three samples in JSONL, `complete=true`, and no observer
process in the cgroup. The human-readable summary also reported stable memory
and RSS, CPU, OOM, I/O, process continuity, and the platform/supervisor context.
The overall observation remained `partial` only because PSI fields were unavailable;
temporary service using `supervise-daemon` showed separate supervisor and child
PIDs in one cgroup. After the child was terminated, OpenRC reported the service
as started with the respawn count increased from zero to one and placed the new
child PID in the same cgroup. A separate run of the built yesh binary crossed a
child restart: the measurement became incomplete without mixing the two
epochs.

The implementation has reached the first working layer. The Linux cgroup
collector is now manager-neutral, yesh parses OpenRC's machine-readable status
and service records, and the OpenRC query links state to PID, `/proc`, and
cgroup. If the child PID disappears during collection, the adapter can use the
`supervise-daemon` process cgroup; the process snapshot is marked incomplete,
and the epoch change is not mixed into one measurement.

## Boundary

Support for Alpine does not mean that yesh must implement OpenRC management,
BusyBox replacements, or every logging format. It means that the core
read-only resource observation has a valid Linux path when systemd and
journald are absent, and that missing context remains visible to the operator.
