# Linux integration evidence

This document records checks that need Linux or POSIX behavior. It is an
evidence record, not a language tutorial and not a list of every runnable
example. The tutorial is in [`syntax-guide.md`](../syntax-guide.md); the practical
scenario catalog is in [`practical-examples.md`](../practical-examples.md).

## Process lifecycle and pipelines

The process tests check that a timeout stops the complete pipeline, an early
reader exit does not leave a writer running, and repeated executions release
file descriptors and child processes.

For example:

```yesh
yes | head -n 1 > /dev/null
```

The rule is deliberate: a failure in any pipeline stage, including SIGPIPE
from the writer, fails the whole command. A successful `head` does not hide a
failed `yes` stage.

The POSIX and Linux process tests cover:

- pipelines with 1, 2, 3, 8, and 32 stages;
- binary streams, slow readers, FIFOs, redirections, and paths with spaces;
- failure of every stage, partial startup, cancellation, timeout, and signal;
- process-group isolation, TERM followed by KILL, direct-child reaping, and
  descendants that retain output descriptors;
- repeated cancellation without accumulating goroutines, descriptors, or
  unreaped children.

The shell fixtures only create test processes. The runtime still starts them
through Go's direct process API; it does not delegate the script to a shell.

## Running the Linux checks

Use caches inside the repository:

```text
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go test ./...
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go test -race ./cmd/yesh
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go vet ./...
GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go build -trimpath ./cmd/yesh
```

The race detector only reports races exercised by the tests. A green run is
evidence for these paths, not a proof of every possible process schedule.

## External tools and real hosts

Tests that need `rsync`, `rsnapshot`, `systemd`, SSH, a mounted filesystem, a
device, or a second UID are marked as environment-dependent. A missing utility
is reported as a skipped test with its required package; a skip is not evidence
that the integration works.

The tests do not install packages, modify production services, change `/boot`,
or use a real remote host during the ordinary suite. Real-host checks must name
the host, operating system, tool versions, paths, and the changes that were
deliberately avoided.

## Service logs and rotation

The canonical service-log scenario is
[`nginx-logrotate-maintenance.yesh`](../../examples/nginx-logrotate-maintenance.yesh).
It treats two independent streams separately: service output is observed through
`systemd-journald`, while text files covered by the active policy are handled by
`logrotate`.

The scenario previews the real policy, requires an operator-created confirmation
file before applying it, and reads a bounded journal window afterwards. It does
not discover ownership, duplicate rotation, clean journald, or restart nginx
merely because text files were rotated. Journal rotation and vacuuming are
separate `journalctl` operations with their own plan and confirmation.

The Debian check used a real `nginx.service`, `/etc/logrotate.d/nginx`, and
`/usr/sbin/logrotate`; the changing run used only a temporary policy and
temporary log tree. The real nginx policy and `/var/log/nginx` were not changed.

## Separate physical-partition check, 2026-11-03

On the stand machine, a separate ext4 partition was mounted at `/opt` through
`/etc/fstab`. Its exact UUID is intentionally not recorded in the repository:
the value must be obtained on the specific machine with `lsblk -f` or `blkid`
and passed to the scenario through the environment.

The partition passed positive `rsync`, an unchanged rerun, changed-file
transfer, wrong-UUID detection, unmounted refusal, and final tree comparison.
Real `rsnapshot` then passed `configtest`, test mode with explicit confirmation,
two rotations, restore of the older snapshot, and unmounted refusal with a
clear message and exit code `3`. yesh does not mount automatically; `fstab` and
directory permissions remain stand-operator responsibilities.

## What this evidence does not prove

These checks do not prove behavior on every kernel, filesystem, distribution,
security policy, hardware device, or power-loss scenario. They also do not prove
that an external service, network, storage device, or utility will succeed after
yesh has completed its own checks. Those boundaries remain part of every
practical example's documentation.
