# yesh roadmap

yesh is an experimental, readable Unix automation language for system administration and
operational automation.

The project priority is:

1. readable code from top to bottom;
2. protection from common operator mistakes;
3. correct Unix/POSIX process behavior;
4. practical administration workflows;
5. carefully justified language extensions.

The detailed engineering plan is available in [Russian](TODO.ru.md).

Russian project notes use Р.П.П. for «реальные практические примеры».
English documentation calls these **Real-World Examples** or **Practical
Examples**. The phrase describes work with real tools or a controlled test
fixture; it does not claim a formal proof or universal production safety.

## Done in the current runtime

- Go runtime with no Python dependency;
- assignments, typed strings, lists, `if`, finite `for`, commands, pipelines,
  redirections, and explicit `exit`;
- direct process execution without reparsing a script through Bash;
- `--plan` for expanded commands and branches;
- command failure handling, process groups, bounded execution, cancellation,
  descriptor cleanup, and pipeline safety checks;
- `exists path` and typed RE2 regular-expression conditions;
- explicit non-empty environment values with masked `--plan` output;
- `free_space path at_least amount` using filesystem statistics and checked
  whole binary units;
- Linux/POSIX tests for SIGPIPE, stuck writers, cancellation, and resource
  cleanup.
- Linux `with lock path:` blocks with non-blocking advisory acquisition and
  release on success, error, cancellation, or script exit.
- `publish source to destination` for same-filesystem rename without a copy
  fallback; crash durability remains separate from namespace atomicity.
- publish tests cover an existing destination directory and a cross-filesystem
  move without losing the temporary source.
- [ ] Add an explicit durability profile with `rsync --fsync`, directory fsync,
  and filesystem-specific crash testing; do not infer power-loss safety from
  rename or process exit alone.
- [ ] Add a filesystem compatibility preflight for backup destinations: record
  `findmnt` filesystem type and mount flags, classify Unix metadata support,
  warn on FAT/exFAT/case-folding/Unicode-normalizing targets, and require an
  explicit reduced rsync profile before destructive actions. Keep hard-link
  capability and `--link-dest` reuse as tested runtime properties, not guesses
  from a filesystem name.
- executable quickstart examples kept in sync across English, Russian and
  the service-check script, with checks for unavailable systemd, failed
  commands, complete pipeline input, predicates and plans.

## What yesh already simplifies

This is the current practical result, not a promise about future syntax:

- readable conditions: ordinary `if` blocks and indentation instead of Bash's
  `[ ]`, `[[ ]]`, `(( ))`, `$`, and historical comparison operators;
- value handling: strings, lists, variables, and regex values remain distinct,
  with no implicit word splitting or re-parsing of values as code;
- execution: the runtime starts argv directly, `--plan` shows expanded
  arguments, and command or pipeline-stage failures remain visible;
- safer repeatable operations: bounded pipelines, timeouts, process and file
  descriptor cleanup, and guarded redirections;
- administration: readable mount/UUID/PARTUUID/free-space checks, SSH/SCP,
  rsync without implicit `--delete`, and read-only process snapshots with RSS
  summaries without hand-parsing `ps` or `awk`;
- bounded systemd service state/restart/wait operations and bounded JSON
  journal queries with explicit manager, timeout, identity, and diagnostic
  boundaries.

The practical claim is deliberately limited: yesh already makes these scripts
easier to read and review, but does not yet claim built-in CIDR/net-tools,
full systemd API, TTY/PTY, typed SSH nodes, or control of other processes.

## Security hardening backlog

The runtime has useful safety guarantees, but they are not a sandbox. Direct
argv execution prevents shell re-parsing; it does not validate the semantics or
authority of every external command. See [SAFETY.md](../SAFETY.md) for the
boundary and operational rules.

- [ ] Define a controlled `PATH` and environment policy for privileged and
  runbook execution.
- [ ] Define an explicit destructive-action policy for `rsync --delete`,
  `--delete-excluded`, `--delete-missing-args`, `--remove-source-files`, and
  recursive removal.
- [ ] Add typed validation or narrow wrappers for high-impact external command
  arguments (`ssh`, `rsync`, `mount`, `systemctl`, and destinations).
- [ ] Expand fuzzing to lexer/parser, value expansion, redirects, and runtime
  cancellation; keep existing identity fuzz cases.
- [ ] Add `govulncheck` and dependency scanning to CI.
- [ ] Add repeat-run allocation/heap profiling for parser, runtime, process
  snapshots, and cancellation paths; monitor child-tool memory separately via
  cgroups/systemd limits.
- [ ] Write a threat model and release security checklist.
- [ ] Run privileged loopback/storage checks only in a disposable VM or
  dedicated Rocky host; never use production disks or the real `/etc/fstab`.
- [ ] Define full OS-level isolation for untrusted yesh scripts.

## Real-world Bash corpus

See the [Bash corpus catalog](bash-corpus.md) for sources and license status.

Before adding language features, evaluate yesh against 20–30 independent
administration scripts. Each entry needs its Bash source, a yesh adaptation,
`--plan`, positive and negative tests, and one concrete reviewability or
safety benefit. A script that merely renames an external command is not
evidence for a new language feature.

- [ ] Select and sanitize backup/rsync, systemd/journal, storage, SSH/SCP,
  and network-check scripts; do not import disabled host-key checks, hidden
  `sudo`, or destructive defaults.
- [x] Adapt an initial set without extending the language: service check,
  read-only storage preflight, backup preview, and SSH health check. These are
  independent examples under `examples/`, with fixture-backed tests in
  `cmd/yesh/practical_posix_test.go`.
- [x] Record the first mismatches with Bash as either an unreadable construct,
  splitting/quoting risk, hidden pipeline failure, or missing safe yesh
  expressiveness in [practical examples](practical-examples.md) and the
  design/safety notes. This is an initial inventory, not a claim that the
  20–30-script corpus is complete.
- [ ] Consider a new built-in only after the same justified problem occurs in
  at least two corpus scenarios.

## Next practical slice: safe transfer and backup

The next major workflow is:

**SSH → SCP → rsync → rsnapshot, with `pv` for observable transfers.**

This work starts with read-only checks, previews, and local test fixtures.
Real remote changes and restore tests belong in a dedicated test VM.

- [ ] typed remote-node description with explicit host, user, and port;
- [x] basic read-only SSH health and SCP argument-vector examples with explicit
  port, timeout, batch mode, and host-key policy;
- [x] use the system SSH client, inherited `SSH_AUTH_SOCK`, OpenSSH config and
  `known_hosts` without reading or storing private keys, passwords, or
  passphrases; test `BatchMode=yes` and no interactive prompt (the opt-in
  real-host test is `TestPOSIXRealSSHAndSCP`);
- [x] explicit host-key policy; never silently allow
  `StrictHostKeyChecking=no`;
- [ ] allow only explicitly selected host-key bootstrap policy; default
  unattended runs to pre-provisioned `known_hosts` and strict verification;
- [ ] when an agent has multiple identities, document/test explicit
  `IdentitiesOnly` selection rather than probing every key;
- [ ] no hidden shell code on the remote host;
- [ ] SCP transfers with source/destination validation and partial-result
  reporting;
- [x] read-only storage preflight example with `findmnt`, `df`, and bounded
  `du`; no implicit mount, unmount, deletion, or cross-filesystem traversal;
- [x] read-only storage identity report with `lsblk`, `blkid`, and `stat`;
- [x] UUID/PARTUUID matching through `findmnt` before an rsync action;
- [x] rsync preview/copy examples with explicit source and destination,
  mount, identity, and free-space checks; no implicit `--delete`;
- [x] confirm rsync-over-SSH manually and with an opt-in test on a separate
  Debian host: a disposable remote source, two snapshots, `--link-dest`, inode
  reuse, and cleanup;
- [x] verify real remote-source failures: rsync code 23 for a read error and a
  missing source directory, no new snapshot, unchanged previous snapshot,
  diagnostic staging, and a successful retry after permissions are restored;
- [ ] separately reproduce rsync codes 24 and 30/35 only after a stable harness
  exists for source disappearance, delay, and connection drop; do not add random
  network races to the required test suite;
- [x] add an opt-in automated real-host rsync-over-SSH integration test with explicit
  remote rsync availability, destination permissions, SSH host-key policy, and
  version/option compatibility (including `--fsync`);
- [x] document remote snapshot semantics: the current incremental example runs
  on the destination host; the source may be local or remote, while `exists`
  and `publish` apply only to the local snapshot directory;
- [x] safe rsync validation on a dedicated ext4 loopback image (positive copy
  and wrong-UUID rejection); real block-partition/PARTUUID validation remains;
- [x] rsnapshot `configtest`, lock conflict handling, retention, and hard-link
  snapshot safety are covered by the local example and integration tests;
- [ ] simplify rsnapshot operation without replacing it: an executable yesh
  wrapper should validate the configured snapshot root/mount and dependencies,
  run `rsnapshot -c CONFIG configtest`, show `rsnapshot -c CONFIG -t LEVEL`,
  and require an explicit apply step for `rsnapshot -c CONFIG LEVEL`;
- [ ] make the rsnapshot wrapper inspect effective configuration before apply:
  surface `rsync_long_args` deletion flags, `sync_first` interval semantics,
  `no_create_root`, lockfile parent permissions, snapshot-root ownership, and
  tab-sensitive config errors;
- [ ] add an opt-in remote-rsnapshot fixture using rsnapshot's existing
  `backup user@host:/path target/`, `cmd_ssh`, `ssh_args`, agent, and
  `known_hosts` configuration. Do not mirror its config grammar or reimplement
  its rotation engine in yesh.
- [ ] rsnapshot mount/source and free-space checks before rotation;
- [ ] `pv` with progress separated from binary data and all pipeline stages
  checked for completion;
- [ ] a shared plan/report showing hosts, paths, commands, volume, and changes;
- [x] tests for broken transfer, lock conflict, reruns, and restore verification;
- [ ] tests for host-key-specific failure, transfer timeout, full disk, and an
  unmounted backup disk.

### Production-like rsnapshot testbed

- [ ] Use a disposable Rocky/Linux VM or host with a dedicated ext4/xfs backup
  filesystem and a known UUID/PARTUUID; never use a production mount.
- [ ] Prepare a real `rsnapshot.conf` with `no_create_root=1`, a private
  lockfile, explicit `retain`, tool paths, and reviewed `rsync_*_args`.
- [ ] Build a fixture containing spaces, binary data, symlinks, hard links,
  changed/deleted files, and enough data to exercise free-space checks.
- [ ] Add an opt-in SSH target with a dedicated account/path, agent-backed key,
  pre-provisioned `known_hosts`, `BatchMode=yes`, and rsync installed on both
  hosts.
- [ ] Run the guarded yesh wrapper through configtest, rsnapshot test mode,
  explicit confirmation, rotation, restore, and cleanup.
- [ ] Exercise the failure matrix: wrong UUID, unmounted root, bad config,
  stale/held lock, missing source, partial transfer, remote SSH failure,
  full filesystem, rerun, and unexpected-delete detection.
- [ ] Record tool versions, filesystem type/mount flags, exact argv, exit
  codes, snapshot contents, and restore checksums; keep the fixture and logs
  outside production paths.

The main verified example is incremental rsync backup with a retention preview.
Physical retention application remains open. The next milestone is a separate
operation that rechecks the root under the lock and moves old snapshots into a
same-filesystem quarantine; the first version must not physically delete
anything. It needs positive and negative tests rather than another wrapper
around one `rsync` invocation. See [the corpus catalog](bash-corpus.md).

Backup direction: make `rsync` and `rsnapshot` materially easier to operate
without replacing either engine. yesh should be an explainable safety layer:
`preflight` → `--plan` → explicit `apply` → publish the completed snapshot →
retention → restore verification. The plan must expose exact argv,
source/destination, lock, `--link-dest`, and potential deletions; after a
failure it must be clear what was published, what stayed unchanged, and
whether retry is safe. For rsnapshot, yesh validates `configtest`, test mode,
mount, and lock, but does not reimplement its rotation engine.

### Next milestone: quarantine old snapshots

- [ ] Define an explicit operation form and the roles of the root, keep count,
  and quarantine directory; do not accept a free-form deletion path.
- [ ] Rebuild the candidate list under the lock and exclude current, previous,
  temporary directories, the lock, symlinks, and unknown names again before
  moving anything.
- [ ] Move candidates only with an atomic same-filesystem rename; a
  cross-filesystem condition must stop the operation.
- [ ] Do not physically delete quarantine contents in the first version; a
  retry must be safe and must not merge directories.
- [ ] Add `--plan`, a positive test, changed-state failure, lock conflict,
  symlink, invalid-name, and rerun cases.

The success criterion for this workflow is operational, not cosmetic: an
operator who knows rsync but not yesh can predict copies and retention moves
before apply, then recover after a failure without guessing. No hidden
`--delete`, implicit rotation, or deletion of an arbitrary path. The first
retention apply must move old snapshots into a same-filesystem quarantine under
the lock; physical purge is a separate explicit action.

The executable slice is now covered by
`TestPOSIXPracticalIncrementalRsyncRoundTrip`: two real local runs, atomic
publication, changed-file verification, and `--link-dest` inode reuse. A
real remote source is additionally covered by
`TestPOSIXRealRsyncPullAndIncremental`. A
separate partial-failure test forces external `rsync` to return 23 after
writing staging: the ready name is not published, the previous snapshot is
unchanged, and a retry acquires the released lock. A held-lock test also
proves that `rsync` is not started and no new snapshot is published. Retention
preview is covered; physical retention and the remaining negative matrix stay
open.

The safe retention preview is now a runtime operation,
`rsync retention plan in $root keep $keep`: it accepts only real
`snapshot.<number>` directories, rejects symlinks, and does not mutate the
filesystem. This is not apply and does not close the full backup plan with
destructive actions exposed.

### Boundaries and next step: incremental rsync

Use only a disposable fixture under `t.TempDir()` or `/tmp`; never touch real
disks, `/etc/fstab`, or production paths. Do not add a
`snapshot` DSL or mirror rsync semantics in yesh.

- [ ] Record yesh/rsync versions, filesystem type and mount flags, exact argv,
  and exit codes.
- [x] Build a fixture with spaces in names, binary data, symlink, hard link,
  changed/new/deleted files.
- [x] Run the first backup into an empty staging directory and publish only
  after rsync succeeds.
- [x] Run the second backup with explicit `--link-dest`; verify tree contents,
  checksums, and inode reuse for unchanged data.
- [x] Restore the second snapshot into a new directory with rsync and compare
  the tree, file types, checksums, and expected links.
- [x] Exercise partial rsync failure: no ready snapshot appears, the previous
  snapshot remains unchanged, and a retry acquires the released lock.
- [x] Exercise missing source, held lock, and rerun; failures do not report
  success, start `rsync` under a held lock, or remove the previous snapshot.
- [x] Add a retention preview: keep the latest N completed snapshots and report
  older ones; physical application is intentionally not implemented yet.
- [x] Verify `--plan`: no rsync, redirect open, or filesystem mutation occurs;
  the report exposes source, destination, lock, retention, and expanded argv.
- [x] Update the documentation, runnable `.yesh` examples, and
  `cmd/yesh/practical_posix_test.go`; record expected failure and no-change
  assertions for the covered negative cases.

The local slice is closed for backup/restore, hard-link reuse, retention
preview, partial failure, missing source, lock conflict, rerun, and plan. The
physical retention action and broader filesystem failure matrix remain open; a
ready name must never point to a partial result.

The product target is a thin backup facade, not a hidden backup engine:
`rsync`/`rsnapshot` remain visible, while yesh makes preflight, lock,
publication, retention, restore, and `--plan` explicit and reviewable.

Do not add a backup-specific `snapshot` DSL before the benchmark. Scoped
locking and same-filesystem publication are generic implemented primitives;
fsync-based crash durability and retention policy remain separate concerns.
Do not mirror `rsync` or `rsnapshot` option sets: utility-specific flags remain
explicit command arguments and their semantics stay owned by those utilities.

The Linux read-only storage interface is implemented:

- `device by uuid|partuuid identity` resolves one device from available lsblk metadata;
- `inspect` displays device or fstab snapshots;
- `fstab path` reads a bounded private snapshot through libmount, rejecting
  parser diagnostics and partial results even with exit status zero;
- `require mounted path from device filesystem type` checks the exact mount;
- `require mounted path from fstab_snapshot` selects one UUID/PARTUUID entry
  and checks device identity, major:minor and filesystem against the kernel mount table.

See [storage](storage.md) and [the runnable check](../examples/fstab-mount-check.yesh).
Plan defers storage reads and queries. Kernel mount lookup disables path
canonicalization and precedes directory inspection. Missing/ambiguous entries,
wrong devices, plain directories and autofs-only targets fail before the next action.
The interface does not write fstab or run mount helpers or daemon-reload.

The last detailed check used WSL with synthetic fstab data and a real root ext4 mount;
PARTUUID and autofs rejection also have fixtures. Real GPT/PARTUUID, automount
activation, backup-mount/restore and boot testing remain unproven. Effective
options, write access, free space and concurrent changes are separate concerns.
Path/LABEL sources, swap, bind/loop/subvolume and auto/type lists are unsupported
by this mount requirement. Whole-filesystem bind mounts cannot be distinguished
from ordinary mounts; uniqueness covers only available device metadata.
Built-in mount/unmount remain planned. The next storage step is to define the
readonly-mount checks and a dedicated test environment.

Storage security decisions for the next slice:

- [ ] keep local UUID/PARTUUID checks separate from network-mount diagnostics;
- [ ] treat `credentials=` as a reference to an external file, never as a
  value to load into yesh; require a regular file, restricted ownership/mode,
  and redact the path/value in reports;
- [ ] reject inline `password=` and never write credentials to `/etc/fstab`;
- [ ] test readonly mount checks, effective options, write access, and free
  space in a privileged Linux environment before adding mount/unmount actions.

- [x] Re-run the full Linux/POSIX/storage/fstab suite on ordinary Linux (not
  WSL) with Go 1.27, including race, vet, and build; the Rocky Linux 9.8 run
  used Go 1.27.3 and passed. Repeat after runtime changes.

The first Real-World Examples in this slice are the runnable
`examples/ssh-health.yesh` and `examples/scp-copy.yesh` pair. Their automated
validation uses fake SSH/SCP tools for argv and failure handling. A real
read-only SSH connection using the same policy and a real SCP copy through
yesh to a separate host were also completed with key authentication and
byte-for-byte checksum verification;
remote restore and partial-result recovery are not claimed yet.

An opt-in real-host test is available for an operator-controlled test VM:
`YESH_SSH_INTEGRATION=1 YESH_SSH_TEST_HOST=user@host
YESH_SSH_TEST_DIR=/tmp/yesh-test go test ./cmd/yesh -run
TestPOSIXRealSSHAndSCP -count=1 -v`. It requires a pre-existing remote
directory and strict `known_hosts` verification; authentication comes from the
normal OpenSSH agent/config, or from `YESH_SSH_IDENTITY_FILE` when explicitly
set. The test copies one unique file and verifies its SHA-256 through SSH.

## Language and runtime backlog

- [x] Regex conditions use Go's standard RE2 implementation: one `matches`
  operator, no backreferences, shell re-parsing, or hidden command execution;
  quoted text such as `"a matches b"` remains data.
- [ ] structured `match` results with named regex captures;
- No new Go dependency is needed for the current regex layer: use the standard
  library `regexp` package. Reconsider only if a concrete requirement proves
  that a third-party dependency improves safety and readability.
- [ ] structured command diagnostics: exit code, signal, timeout, stderr,
  duration, and failed pipeline stage;
- [ ] one consistent builtin execution model, then `cd` and environment
  mutation operations (`export` and process-local changes);
- [ ] bounded file search and safe, planned atomic replacement;
- [x] read-only exact-name process snapshots with bounded `/proc` scanning and
  fixed-field `show` output; signals and broader filters remain;
- [x] verified the snapshot against a real Rocky Linux nginx service: five
  processes, master UID 0, workers UID 979, `/system.slice/nginx.service`, and
  RSS total/max/average; the captured result is recorded in
  `docs/practical-examples.md`;
- [ ] typed process diagnostics beyond this read-only slice;
- [x] read-only exact-unit systemd status from an allowlisted `systemctl show`
  snapshot, with explicit system/user manager selection and plan output;
- [x] bounded `journalctl` unit query with JSON validation, output limits,
  stderr diagnostics and explicit empty-result semantics;
- [x] bounded `service wait` and restart with an `active` postcondition,
  canonical-id checks, timeout diagnostics and restart snapshot fields;
- [x] Real disposable Debian/systemd fixture over SSH: `start`, `status`,
  `wait`, `restart`, terminal `oneshot`, activating timeout, and failed unit
  with exit status 23; cleanup left no fixture units or sleep processes.
- [ ] distinguish `oneshot` completion from `active`, queued-job semantics and
  application readiness probes; do not claim that manager state is readiness;
- [ ] model `LoadState=masked` as an actionable service state: report it in
  status, fail `start/restart/ensure active` clearly, and never auto-unmask;
  test an explicit unmask operation separately if it is added later;
- [x] typed read-only device resolution, fstab snapshots and exact active-mount
  checks, including binding one fstab entry to its UUID/PARTUUID device;
- [ ] effective mount-option and specialized filesystem diagnostics beyond
  the supported whole-filesystem identity/type check;
- [ ] add explicit, preflight-checked `mount`/`unmount` operations only after
  device identity, mountpoint, timeout, busy-state, and wrong-device tests;
- [ ] state files and resumable long-running jobs only after safe cancellation
  and ownership boundaries are established;
- [ ] full isolation for untrusted scripts through operating-system controls.

The process boundary is intentional: yesh currently reads a bounded `/proc`
snapshot and aggregates RSS, while its runtime manages its own child processes.
It does not stop nginx or other foreign processes, expose a general `kill` or
`pkill` operation, mutate cgroups, or claim full systemd service control yet;
the bounded typed systemd operations are listed above.

## Networking and CIDR

The networking layer is primarily for readable preflight checks before SSH,
SCP, and rsync. It is not intended to replace iproute2 or silently configure
the host network. It is one of the first practical layers after the current
documentation and diagnostic work.

- [ ] Typed IPv4/IPv6 addresses and CIDR prefixes using Go's standard
  `net/netip`: strict parsing, normalization, `contains`, and explicit
  IPv4/IPv6/mapped-IPv6 family rules; no hand-written mask arithmetic.
- [ ] A readable address-in-network condition with negative examples and
  explicit errors for invalid masks, addresses, or mixed address families.
- [ ] Read-only interface/address reports through checked `ip` invocations;
  missing tools, permission failures, and incomplete output must remain visible.
- [ ] Read-only ARP/neighbour report with IP, MAC, state, and interface;
  a missing entry must not be treated as proof that a host is unreachable.
- [ ] Read-only route and TCP/UDP listener reports through `ip` and `ss`, with
  bounded output and no parsing of unstable decorative text without an
  explicit format.
- [ ] Bounded DNS, TCP-port, and ICMP checks with explicit timeout/count;
  ICMP absence must not be treated as proof that a host is unreachable.
- [ ] Define check semantics: a normal negative result (`port is closed`,
  `address is outside the CIDR`) is `false`; missing tools, timeout,
  permission failure, or malformed output are execution errors. Built-in
  checks must not invent an arbitrary exit-code table; the script author
  explicitly chooses the final `exit` and its code.
- [ ] Keep the first version safe: no built-in changes to addresses, routes,
  firewalls, namespaces, or interface state.
- Legacy `net-tools` commands (`ifconfig`, `route`, `arp`, `netstat`) are not
  embedded or required; start with modern `ip`/`ss` from iproute2 and checked
  arguments.
- No new Go networking dependency is needed for CIDR. Consider netlink only
  if external `ip`/`ss` cannot provide a stable, testable interface.

## Design rule

A feature belongs in yesh only when a real administrative example demonstrates
that it is easier to read, review, and safely repeat than the Bash equivalent.

The detailed Russian plan may contain research notes and future ideas. It must
not make a planned interface look like a supported runtime feature.
