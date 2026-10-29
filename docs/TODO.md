# yesh roadmap

yesh is an experimental, readable Unix shell for system administration and
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
  summaries without hand-parsing `ps` or `awk`.

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
- [ ] Add typed validation or narrow wrappers for high-impact external command
  arguments (`ssh`, `rsync`, `mount`, `systemctl`, and destinations).
- [ ] Expand fuzzing to lexer/parser, value expansion, redirects, and runtime
  cancellation; keep existing identity fuzz cases.
- [ ] Add `govulncheck` and dependency scanning to CI.
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
- [x] safe rsync validation on a dedicated ext4 loopback image (positive copy
  and wrong-UUID rejection); real block-partition/PARTUUID validation remains;
- [x] rsnapshot `configtest`, lock conflict handling, retention, and hard-link
  snapshot safety are covered by the local example and integration tests;
- [ ] rsnapshot mount/source and free-space checks before rotation;
- [ ] `pv` with progress separated from binary data and all pipeline stages
  checked for completion;
- [ ] a shared plan/report showing hosts, paths, commands, volume, and changes;
- [x] tests for broken transfer, lock conflict, reruns, and restore verification;
- [ ] tests for host-key-specific failure, transfer timeout, full disk, and an
  unmounted backup disk.

The first full benchmark case is incremental rsync backup with retention. It
is intentionally still open: the adaptation must cover lock/state handling,
preview, partial failure, rerun behavior, and restore verification rather than
just wrap one `rsync` invocation. See [the corpus catalog](bash-corpus.md).

The product target is a thin backup facade, not a hidden backup engine:
`rsync`/`rsnapshot` remain visible, while yesh makes preflight, lock,
publication, retention, restore, and `--plan` explicit and reviewable.

Do not add a backup-specific `snapshot` DSL before the benchmark. The first
language candidates, only if confirmed by at least two workflows, are generic
scoped locking and atomic publication of a completed temporary result.
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
- [ ] typed process and systemd diagnostics beyond this read-only slice;
- [ ] read-only systemd service status from stable `systemctl show` fields and
  bounded `journalctl` output;
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
`pkill` operation, mutate cgroups, or claim systemd service control yet.

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
