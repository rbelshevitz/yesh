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

### Static release publication

After `v0.2.2`, the project verified static release publication without adding
new yesh syntax. Three examples cover nginx under OpenRC, lighttpd under OpenRC
on Alpine, and nginx under systemd. The checked order is:

```text
prepared tree → required file → server configuration → lock
→ publication → current switch → HTTP check
```

The Linux fixtures cover successful publication, an HTTP failure, rollback, a
held lock, invalid server configuration, an existing temporary symlink, reruns,
and interruption after `current` has switched. The latter establishes the
boundary that ordinary rollback is compensating work, not a transaction.

The later polish flattened deeply nested validation in the three local scripts
without changing exit codes or lock release. After a failed HTTP check, each
script restores the previous symlink first and then checks the restored release
through the same URL. That second check observes the result; it does not decide
whether restoration is attempted. `deploy-release-rwe.yesh` remains a separate
teaching exercise with an unconditional rollback, rather than a health-check
failure path.

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
- local service resource history: finite `measure`/`summarize`, a JSON Lines
  journal, `keep history` retention, memory/RSS/CPU/OOM and block-I/O comparison,
  incomplete tails, corrupt records, lock conflicts, and segmentation after an
  `InvocationID` change.

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
- a real ARM64 execution check.

Older speculative designs, duplicated milestone lists, and completed
intermediate checklists were removed from this archive when the short roadmap
was compacted. The repository history remains the source for exact revisions.

## Moved from the working roadmap after `v0.2.2`

The detailed `v0.2.1` acceptance and stand matrix are closed and no longer
occupy the current roadmap. Their results remain in the confirmed-evidence
section and in Git.

The architecture review did not add built-in network, container, or monitoring
capabilities. The next practical candidates are configuration publication,
pre-publication network checks, replacement of one container service, and
finite Alpine/OpenRC diagnosis. They must first use existing external commands,
explicit argv, locks, timeouts, `--plan`, and result checks. A new capability is
allowed only if the same problem recurs in at least two independent scenarios.

Second-tier candidates are a one-shot SQLite migration, rootless container
volume preparation, initramfs structure checks, and log maintenance. A permanent
daemon, node discovery, a distributed desired-state model, built-in DNS/TLS/HTTP/
firewall, and Docker/Podman management remain outside the project boundary.
