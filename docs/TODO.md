# Current yesh roadmap

This is the short working map. It contains only the current milestone,
verifiable tasks, and boundaries. Historical decisions and closed discussions
are kept in the [archive](TODO-history.md).

## Current milestone: `v0.2.0`

The goal is to make the implemented Linux scenarios understandable and
reproducible. New language constructs are not the goal of this milestone. The
acceptance boundary is Linux amd64.

The release must include:

- backup and restore with result verification, locking, reruns, failure, and
  `--plan` coverage;
- visible `rsync` and guarded `rsnapshot` workflows with real external argv and
  explicit limits;
- configuration publication followed by service and bounded journal checks;
- separate `logrotate` and journald ownership without hidden double rotation;
- a finite `service resources` snapshot for systemd and cgroup v2;
- finite `measure service` with bounded JSONL history and `summarize`, including
  unit, invocation, and cgroup identity plus incomplete observations;
- consistent Russian explanations, English examples, manuals, and links;
- reproducible Linux checks, a release build, checksums, and clear limits.

This milestone does not promise general monitoring, a continuous agent,
long-running-job resume, cgroup management, ARM64, or replacements for
`rsync`, `rsnapshot`, systemd, SSH, and other external tools.

## Confirmed

- The parser and runtime support assignments, strings, lists, `if`, `for`,
  commands, pipelines, redirects, `--plan`, `exists`, regular expressions,
  and explicit `exit code`.
- Processes run directly through the Go runtime with bounded execution,
  cancellation, and child cleanup.
- Advisory locking and same-filesystem `publish` without hidden copy fallback
  are implemented.
- Read-only typed operations for storage/fstab, systemd/journal, and resource
  snapshots remain bounded; `mount`, `umount`, `start`, `stop`, `reload`, cgroup
  management, and control of foreign PIDs are not implemented.
- On a separate Debian node, SSH/SCP and real `rsync` passed in both directions.
  The loopback/ext4 `rsnapshot` stand passed with `no_create_root`, two
  rotations, hard links, restore, `--plan`, negative checks, and cleanup.
- Debian checks covered resource snapshots and finite service measurement.
  Different invocations are not combined; unavailable counters remain explicit
  rather than becoming zero.
- A separate Debian stand passed the complete configuration-publication
  scenario: `--plan`, atomic publication, systemd restart and verification,
  bounded journal output, and invalid-configuration failure without changing
  the active file.
- The local real `rsnapshot` stand passed `--plan`, two rotations, hard-link
  reuse, restore, locking, and cleanup; the stand accounts for WSL2's `/tmp`
  source representation.
- The `logrotate` maintenance test passed with separate state and bounded
  journal checking without restarting the service.
- The `v0.2.0` release build passed locally: full `go test`, `go vet`, a
  Linux/amd64 binary with injected version, `--version` and `-h` checks, an
  archive with README files and the license, and a SHA-256 checksum.
- Local Markdown links were checked after moving deep material to
  `docs/underthehood`.

These results prove specific Linux/amd64 stands, not every OS, filesystem,
network, service, or power-loss scenario.

## `v0.2.0` acceptance

Run the acceptance in this order:

1. incremental `rsync` from a source on another node;
2. a guarded `rsnapshot` interval;
3. a finite service-resource snapshot;
4. snapshot comparison with a break at service restart;
5. configuration publication and service-state verification;
6. separate file-log and journald maintenance.

Each scenario needs a positive case, a negative case, a rerun, `--plan`, a
machine-readable report, and a check that failure leaves data unchanged.
External commands remain visible: yesh does not hide their argv, exit codes, or
responsibility for the network, permissions, and filesystem.

### Stand matrix

- local temporary stand without privilege: two `rsync` snapshots,
  `--link-dest`, rerun, failure, lock, publication, and restore;
- a real SSH node: transfer in both directions, read failure, and rerun;
- a separate Linux loopback/ext4 stand with root: `rsnapshot`,
  `no_create_root`, two rotations, hard links, restore, and an unmounted
  destination.

The root check is run manually on a separate node. yesh does not escalate
privileges and does not present a local skip as proof of real SSH or mounting.

## Immediate tasks

### 1. Polish the backup boundary

Review implementation, scenarios, and diagnostics around source and destination,
temporary trees, publication, rsync codes 23/24/30/35, `--link-dest`, explicit
`--delete`, locking, reruns, and temporary files.

The tutorial must explain SSH keys: the private key stays on the yesh machine,
the public key is installed on the other node, the host fingerprint is checked
through a trusted channel, and `BatchMode=yes`, `ConnectTimeout=5`, and
`StrictHostKeyChecking=yes` prevent password waits or silent trust of a new
host.

💡 Problem solved by yesh: the operator can see the whole “check → transfer →
  verify → publish” boundary instead of reconstructing it from fragile quoting
  and shell defaults.

### 2. Polish `rsnapshot`

Keep `rsnapshot` responsible for transfer and rotation. Before the selected
interval, show and check `configtest`, test mode, destination, and mounting; for
removable destinations retain `no_create_root=1`.

🚫 Danger zone: an ordinary directory in place of the mounted destination,
  `sync_first`, stale lockfiles, `.sync` or `_delete.*`, and publication after
  a partial transfer.

### 3. Polish service observation

Keep the boundary “finite snapshot, not monitoring”. Check a positive snapshot,
an unavailable optional counter, a disappearing process, a changed cgroup tree,
and a restart with a new `InvocationID`. `summarize` must not combine different
service invocations or present process RSS as `memory.current`.

🚫 Do not add a daemon, alerting system, `pkill`, `cgroup.kill`, limit writes, or
  automatic container ownership claims here.

### 4. Polish tutorial and manuals

Every practical chapter must state:

1. which concrete administrator problem yesh solves;
2. which repeated mechanics it removes or makes visible;
3. what remains the responsibility of the external command, host, and
   filesystem;
4. what the positive case, failure, and `--plan` show;
5. which boundary the stand proved.

Place a short Bash equivalent beside the example for comparison only. Russian
prose stays Russian; commands, flags, filenames, JSON fields, and keywords stay
exact. Explain a necessary English term on first use and then choose the form
that best fits the context.

💡 In the tutorial, `💡 ` marks yesh's benefit, `🚫 ` marks a danger or required
  check, and `❌ ` marks a concrete failure or prohibited option. The adjacent
  prose must remain understandable without emojis.

## Do not do before the milestone is complete

- do not add `observe`, `resume`, a general desired-state language, or new
  network modules;
- do not turn finite measurement into a permanent metrics collector;
- do not rewrite `rsync`, `rsnapshot`, `systemctl`, `mount`, SSH, or a package
  manager as built-ins;
- do not expand shell compatibility (`cd`, `export`, command substitution,
  TTY/PTY) without two independent practical scenarios;
- do not start Darwin/macOS or present cross-compilation as ARM64 validation.

A new feature is allowed only after answering what real scenario it simplifies,
which ambiguity it removes, what state model it introduces, and how positive,
negative, and `--plan` cases prove the benefit.

## Next working step

The local release check is closed: the tag workflow now repeats the practical
dependencies, full tests, and `vet` before packaging. The only open action is
to create the tag and publish after reviewing the final diff. Do not add
network capabilities or metrics before that step.

## Boundaries and documents

yesh is not an interactive shell, backup product, inventory system, Ansible,
Python, or replacement for external Linux tools. The project formula is
**Linux automation you can explain**.

- [language tutorial](syntax-guide.md)
- [practical examples](practical-examples.md)
- [Linux integration results](underthehood/linux-integration.ru.md)
- [deep technical material](underthehood/README.md)
- [safety boundaries](../SAFETY.md)
- [Russian roadmap](TODO.ru.md)
- [decision and verification archive](TODO-history.md)
