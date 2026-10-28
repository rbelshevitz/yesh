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

## Next practical slice: safe transfer and backup

The next major workflow is:

**SSH → SCP → rsync → rsnapshot, with `pv` for observable transfers.**

This work starts with read-only checks, previews, and local test fixtures.
Real remote changes and restore tests belong in a dedicated test VM.

- [ ] typed remote-node description with explicit host, user, and port;
- [x] basic read-only SSH health and SCP argument-vector examples with explicit
  port, timeout, batch mode, and host-key policy;
- [ ] use the system SSH client, agent, and config without reading or storing
  private keys, passwords, or passphrases;
- [ ] explicit host-key policy; never silently allow
  `StrictHostKeyChecking=no`;
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
- [ ] rsnapshot `configtest`, locking, mount checks, retention, free-space
  checks, and hard-link snapshot safety;
- [ ] `pv` with progress separated from binary data and all pipeline stages
  checked for completion;
- [ ] a shared plan/report showing hosts, paths, commands, volume, and changes;
- [ ] tests for host-key failure, timeout, broken transfer, full disk,
  unmounted backup disk, locks, reruns, and restore verification.

The storage examples above are read-only preflight checks. yesh does not yet
provide a built-in `mount`, `unmount`, `require mounted`, device-resolution, or
`fstab` API; those remain planned safety work and must not be treated as a
root-capable interface.

The first Real-World Examples in this slice are the runnable
`examples/ssh-health.yesh` and `examples/scp-copy.yesh` pair. Their current
validation uses fake SSH/SCP tools and a local source fixture. A local loopback
SSH/SCP transfer was also completed manually and verified byte-for-byte; a
copy to a separate remote machine or a restore workflow is not claimed yet.

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
- [ ] typed process and systemd diagnostics beyond this read-only slice;
- [ ] read-only systemd service status from stable `systemctl show` fields and
  bounded `journalctl` output;
- [ ] extend typed storage, mount, and `fstab` diagnostics beyond the current
  read-only examples;
- [ ] add explicit, preflight-checked `mount`/`unmount` operations only after
  device identity, mountpoint, timeout, busy-state, and wrong-device tests;
- [ ] state files and resumable long-running jobs only after safe cancellation
  and ownership boundaries are established;
- [ ] full isolation for untrusted scripts through operating-system controls.

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
