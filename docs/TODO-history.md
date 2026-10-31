# yesh archived decisions and evidence

This file records decisions and completed evidence that no longer belongs in
the short working roadmap. It is historical context, not a list of promises.

## Product boundary

The public position is **Linux automation you can explain**. yesh is a small
language and Go runtime for readable administrative workflows. It does not
replace Bash as an interactive shell, Python as a general language, Ansible as
an inventory/orchestration system, or `rsync`/`rsnapshot`/`systemctl`/`mount` as
external tools.

The design is intentionally narrow: one block form for control flow, explicit
`$name` expansion, separate strings and lists, direct process execution, and
failure as an error by default. New built-ins require two independent practical
scenarios, an example, a plan or diagnostic, positive and negative checks, and
an explicit evidence boundary.

## Frozen language and runtime evidence

The parser/runtime test corpus covers assignments, lists, conditions, loops,
pipelines, redirects, variable boundaries, regular expressions, `exists`,
`exit code`, locks, publication, cancellation, time limits, and `--plan`.
Commands are executed through the Go runtime; the whole script is not handed
to Bash and values are not reparsed as shell text.

`--plan` does not execute commands or open redirected output. It shows exact
external `argv`; built-ins such as `publish` are shown as named yesh actions.
On POSIX, pipeline failure cancels the process group and waits for children.
These claims are limited by the host kernel, utilities, permissions, and
filesystem.

## Completed practical evidence

- archive and restore workflows with `tar`, `gzip`, and `pv`;
- configuration validation, staging, same-filesystem publication, service
  restart with an active postcondition, bounded journal, and failure paths;
- local and remote `rsync` preflight, incremental snapshots, `--link-dest`,
  partial transfer, lock conflict, retry, restore, and unchanged previous state;
- `rsnapshot` configuration validation, test mode, two rotations, hard-link
  reuse, restore, lock handling, loopback/ext4 mounting, and cleanup;
- SSH/SCP checks with explicit port, timeout, batch mode, host-key policy, and
  checksum verification;
- read-only storage/fstab checks, mount identity, free-space checks, process
  snapshots, systemd/journal reports, initramfs inspection, cron and package
  queries, and network observation through visible external tools;
- disposable Debian, WSL2, Rocky Linux, loopback-ext4, and isolated KVM
  environments. Real devices, production paths, and power-loss durability are
  not covered by these fixtures.

The repository contains more than 200 automated test functions and four fuzz
targets. A test skip for a missing external tool is not evidence that the
integration works. Root, a real systemd manager, SSH, a device, a second UID,
and other host capabilities are labelled as environment-dependent.

## Security and operational limits

yesh is reviewable tooling, not an isolation boundary. Direct execution of a
renamed or malicious executable is outside the language safety contract; shell
wrapper rejection is only a supporting measure. yesh does not add `sudo`,
install packages, change arbitrary services, control foreign PIDs, or promise
rollback of completed external actions.

Atomic same-filesystem rename controls name visibility, not crash durability.
`rsync --fsync` does not prove directory fsync or survival of power loss.
`rsnapshot` remains the rotation engine; yesh checks its prerequisites, mode,
lock, and publication boundary without copying its configuration language.

## Portability and release record

Portable parser/runtime tests and builds run on Windows CI; Linux CI covers
Linux/POSIX behavior. Linux-specific files have portable stubs where required.
macOS/Darwin is deferred.

The first public development release is `v0.1.0` for Linux amd64, built with
Go 1.21 and `CGO_ENABLED=0`. ARM64 cross-compilation succeeds, but runtime
support is not claimed until a real ARM64 smoke test is completed.

## Deferred work

- production-like rsnapshot/storage testing on a separate filesystem;
- explicit durability profile and directory-fsync policy;
- controlled `PATH` and environment for privileged execution;
- durable job identity and resume protocol;
- broader systemd/cgroup, TTY/PTY, boot, NFS/FUSE, and real-device testing;
- independent review of the tutorial and a real ARM64 execution check.

Older speculative designs, duplicated milestone lists, and completed
intermediate checklists were removed from this archive when the short roadmap
was compacted. The repository history remains the source for exact revisions.
