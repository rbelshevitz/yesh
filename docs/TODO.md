# Current yesh roadmap

This is the short working map. It contains only the current milestone,
verifiable tasks, and boundaries. Historical decisions and closed discussions
are kept in the [archive](TODO-history.md).

## Current focus: after `v0.2.2`

Releases `v0.2.1` and `v0.2.2` are published. `v0.2.2` is a technical release
for polishing the implemented Linux scenarios, documentation, and checks. New
language constructs are out of scope for now. The working boundary is
Linux/amd64.

### `v0.2.2` acceptance

- [x] systemd and experimental OpenRC support are described in the README,
  tutorial, and manual;
- [x] OpenRC was checked on Alpine: platform, supervisor, service state, cgroup,
  processes, and finite JSONL measurement;
- [x] the Windows build preserves the common OpenRC shape and reports the
  Linux-only boundary explicitly;
- [x] the incremental `rsync` Bash baseline was written, reviewed, and checked
  with positive and negative disposable scenarios;
- [x] the benchmark includes a concrete Bash/yesh comparison;
- [x] full release run: `go test ./...`, `go vet ./...`, a `-trimpath` build,
  and green CI;
- [x] review release text and mark OpenRC as experimental support;
- [x] publish tag `v0.2.2` after reviewing the file list and checksum.

`v0.2.2` does not claim production-ready support for every Linux system. It does
not include supervisord, container diagnostics, continuous monitoring, ARM64, or
an independent user audit. `v0.3` starts only after three paired Bash/yesh
reference scenarios: backup, configuration publication, and service diagnosis.
The navigator is not abandoned: it remains a separate experimental companion
milestone and is intentionally outside the core `yesh` command contract.

Release `v0.2.0` already includes:

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
- The storage checks are sufficient for the backup boundary: yesh verifies the
  mount, expected source, and filesystem with `findmnt`, space with `df`/
  `free_space`, and path properties with `stat`, then stops before `rsync` or
  `rsnapshot` when the destination is not proven. Mounting and unmounting stay
  outside `v0.2.0`.
- `storage-preflight.yesh` was checked with a placeholder UUID: `--plan` only
  shows deferred checks, while a real run stops during device resolution and
  does not attempt to mount anything. A separate physical ext4 partition
  mounted at `/opt` through `fstab` then passed the positive run, unchanged
  rerun, changed-file transfer, wrong-UUID refusal, and unmounted refusal; the
  final tree matched the source. The UUID remains a stand-specific parameter,
  not a repository example.
- On a separate Linux stand, `backup-mounted-rwe.yesh` passed positively:
  loopback/ext4 with a real UUID, backup write, `--plan`, wrong UUID,
  read-only, unmounted target, and low-space cases. All negative checks left
  the target unchanged.
- On a separate Debian node, SSH/SCP and real `rsync` passed in both directions.
  The loopback/ext4 `rsnapshot` stand passed with `no_create_root`, two
  rotations, hard links, restore, `--plan`, negative checks, and cleanup; both
  normal mode and `sync_first=1` (`sync` → rotation) were verified separately.
  The current `yesh-rsnapshot-mounted-rwe.sh` itself also passed with the
  expected `daily.0`/`daily.1` contents and restore output.
- A second physical partition is not required for this acceptance: the
  temporary loopback/ext4 image is sufficient. A real destination such as
  `/var/cache/rsnapshot` still requires its own mounted filesystem; a directory
  on the root filesystem is not an accepted backup destination.
- The real-partition `rsnapshot` check is complete: the UUID,
  `no_create_root=1`, two rotations, restore of the older snapshot, refusal
  while unmounted with “snapshot root is not mounted” and exit code `3`, and a
  successful run after remounting were verified. Before the preflight-order
  fix, yesh leaked a low-level `stat` permission error; it no longer does.
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

## `v0.2.1` acceptance

Status: the current corpus is accepted against the confirmed Linux/amd64
scenarios. Stand results do not claim universal support for every kernel,
distribution, filesystem, or security policy.

- [x] incremental `rsync` from a source on another node;
- [x] a guarded `rsnapshot` interval;
- [x] a finite service-resource snapshot;
- [x] snapshot comparison with a break at service restart;
- [x] configuration publication and service-state verification;
- [x] separate file-log and journald maintenance.

Before publishing the tag, repeat only the release checks:

1. `go test ./...`;
2. `go vet ./...` and a `-trimpath` build;
3. green Linux and Windows CI;
4. review the change list, README files, manuals, and release text.

The acceptance was based on positive and negative cases, reruns, `--plan`,
machine-readable reports, and checks that failures leave data unchanged.
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

yesh lets you build reviewable backup scenarios around standard Linux tools:
`rsync` and `rsnapshot`.

If a transfer fails, yesh does not publish the incomplete backup. The last
successfully created backup remains available.

`rsync` performs transfers and can restore a selected snapshot. `rsnapshot`
performs transfers and snapshot rotation. A restore command remains external.
yesh does not copy or restore data itself. It prepares and checks the scenario:
it verifies the destination, defines the order, locks the run, keeps incomplete
trees unpublished, and checks the result. This makes external command execution
predictable and controllable without hiding the tools' responsibility.

Add a separate backup-result check to the practical example: verify expected
files/content rather than only the `rsync` exit code, then perform a restore
drill into an empty temporary tree. Teach the order `--dry-run` → review
changes → execute → restore. Keep `--delete` an explicit operator decision;
yesh must not infer it from synchronization or make a mirror the default
meaning of backup.

### 2. Polish `rsnapshot`

Keep `rsnapshot` responsible for transfer and rotation. Before the selected
interval, show and check `configtest`, test mode, destination, and mounting; for
removable destinations retain `no_create_root=1`.

A navigator flow for `rsnapshot` is allowed as a preflight-and-run wizard, not
as an editor for `rsnapshot.conf`. It must distinguish a normal interval from
`sync_first`: in the latter mode it must show and run `rsnapshot sync` first,
then a separate rotation interval. Before execution it must show the plan,
require explicit confirmation, and preserve the external `argv`; `retain`,
`sync_first`, `no_create_root`, `lockfile`, and backup sources remain the
responsibility of the `rsnapshot` configuration and operator.

Add an explicit navigator check for the chain form → real parser → `--plan` →
optional review window → save. Do not treat a selected `daily` as a data
transfer until it is established that `sync_first` is disabled.

The tutorial has been proofread against the real `rsnapshot` stand: it explains
that `configtest` checks configuration syntax and helper paths but does not
guarantee a successful transfer; `--delete` and `--delete-excluded` come from
the standard configuration, remove only destination-side extras, and require
review of the test report. It shows the boundary `--plan` → test report →
explicit confirmation file → rotation → restore, including `/./`,
`daily.0`/`daily.1`, hard-link verification, and the fact that `fstab` and
automatic mounting are outside this scenario.

🚫 Danger zone: an ordinary directory in place of the mounted destination,
  `sync_first`, stale lockfiles, `.sync` or `_delete.*`, and publication after
  a partial transfer.

### 3. Polish service observation

Keep the boundary “finite snapshot, not monitoring”. Check a positive snapshot,
an unavailable optional counter, a disappearing process, a changed cgroup tree,
and a restart with a new `InvocationID`. `summarize` must not combine different
service invocations or present process RSS as `memory.current`.

The first technical OpenRC layer is now prepared:

- [x] the common cgroup collector is independent of systemd;
- [x] machine-readable `rc-status -C -f ini -a` parsing;
- [x] positive and negative parser tests;
- [x] parsing the service record from `/run/openrc/daemons/<service>/001`;
- [x] extending JSONL records with platform, supervisor, and process epoch;
- [x] an initial OpenRC adapter linking ordinary service state to verified PIDs
  and cgroup;
- [x] automatic confirmed-source selection for `status`, `resources`, and
  finite `measure`;
- [x] a dedicated `supervise-daemon` proof with a new child-process epoch;
- [x] `summarize` accepts OpenRC JSONL and emits the same memory, RSS, CPU, OOM,
  and I/O summary with platform, supervisor, and an explicit incomplete-epoch
  marker;
- [x] run that summary on the clean `lighttpd` stand and verify its output;
- [ ] OpenRC user-service validation with `XDG_RUNTIME_DIR`.

Mandatory architectural rule: observe resources at the Linux level (`cgroup
v2` and `/proc`), with systemd as only the first target adapter. Do not make
systemd or journald prerequisites for general measurement: Alpine/OpenRC and
minimal systems must remain valid targets with an explicitly absent service-
state or journal layer.

Before selecting an adapter, identify the local OS from the `ID` field in
`/etc/os-release` (falling back to `/usr/lib/os-release`), but do not infer a
working OpenRC from `ID=alpine`. Separately confirm an operational service
manager interface and state (systemd or OpenRC), then verify the service, PID,
and cgroup. Use `PRETTY_NAME` only for display and `ID_LIKE` only as a family
classification hint. In a container, do not treat `/run/host/os-release` as
the current environment's OS. If the probes disagree, do not guess the manager.

The first common layer should stay simple: one service, automatic source
selection, and an explainable report. Use machine-readable interfaces for
reading: `systemctl show` and `rc-status` with `ini` output and `-a`. Do not
make the script author choose a
manager or supervisor. Treat `supervise-daemon` as an OpenRC implementation
detail. The report must still show reported state, PID, process start time,
cgroup, and process-epoch changes separately. Keep the common contract limited
to status reading, a resource snapshot, and finite measurement for now.

### Containers with `supervisord`

This branch is not part of current support and must not be described as ready.
The decision is to keep developing it only as a separate experimental,
read-only container adapter after the common systemd/OpenRC contract and the
`v0.3` reference scenarios are stable. It is not a reason to broaden the core
service language now.

Keep this as a separate testable branch because a container PID and a host PID
may differ. A PID number is not enough identity: combine the PID namespace,
`/proc` `starttime`, cgroup evidence, and `supervisord` XML-RPC as separate
evidence layers.

- [ ] read `getProcessInfo`/`getAllProcessInfo` through an authenticated Unix
  socket without adding process management;
- [ ] compare Supervisor state with `/proc` and cgroup, without treating a
  Supervisor group as a cgroup resource boundary;
- [ ] distinguish yesh running inside the container, in a diagnostic container,
  and on the host with namespace access;
- [ ] mark the result incomplete when child processes or cgroups are hidden;
- [ ] test a restart with a new identity tuple: PID, `starttime`, and PID
  namespace;
- [ ] add positive, negative, and `--plan` scenarios.

Do not add process management, restart/stop operations, a daemon, or a
continuous monitor for this branch. A first useful result is one finite report
that combines authenticated Supervisor state with `/proc` and cgroup evidence,
and marks hidden namespaces or missing children as incomplete. Do not call this
"supervisord support" until that report passes on a disposable container
fixture.

Boundary: `RUNNING` from `supervisord` confirms its process model, but does not
prove resource ownership, complete container visibility, or application health.
yesh must not infer that a container owns host processes.

This remains useful on large systems, but not as a replacement for central
monitoring. Across a large fleet, yesh should stay a finite diagnostic scenario
for one node, one service, or one incident: check “configured → started →
controller → PID → cgroup → resources → log source” and preserve readable
evidence. This is useful after a configuration change, an unexpected restart,
on a node without a monitoring agent, or when central state disagrees with the
actual node. An external tool such as SSH or fleet management may run the
scenario on many nodes; yesh must not become the agent, time-series database,
or alerting system itself.

- [ ] define a finite “single-service diagnosis” report with explicit evidence
  layers and `no data`/`unsupported` states;
- [ ] check the same report on one systemd node and one OpenRC node;
- [ ] add a separate example that runs the scenario on a selected node through
  external SSH, without adding a hidden remote executor to yesh;
- [ ] document the boundary between local yesh diagnosis and central fleet
  monitoring.

🚫 Do not add a daemon, alerting system, `pkill`, `cgroup.kill`, limit writes, or
  automatic container ownership claims here.

### 4. One container service on a small node

This is not a new “container” niche. yesh's general niche is a finite,
verifiable operation on one Linux node where external tools already do the work
but shell glue hides ordering, failure boundaries, and result checks. The
container case is the next concrete example after static release publication.

The first stand should replace one service without an orchestrator:

```text
verify image by digest → start the new container beside the old one
→ check state and HTTP → switch the external entry point
→ remove the old container only after success
```

Keep Docker/Podman, the registry, volumes, networking, nginx, and lighttpd as
external owners. yesh supplies visible `argv`, locking, conditions, plans,
error codes, and result checks. Before the switch, the old state must remain
working; after the switch, any return is a compensating rollback, not a
transaction.

- [ ] Debian or Alpine with two local containers from one small image;
- [ ] nginx or lighttpd in front of two local ports;
- [ ] rootless Podman and a separate Docker run, with the privilege boundary
  documented;
- [ ] success, failure before switching, failure after switching, held lock,
  rerun, stale temporary object, and `--plan`;
- [ ] image admission by digest and a separate DNS/TLS/HTTP preflight;
- [ ] no new `container run`, OCI, DNS/TLS, firewall, signature-policy, or
  Kubernetes built-in.

#### Further image and network scenarios

Prioritize scenarios that connect several external evidence sources and refuse
before changing state:

1. **Image admission:** `skopeo inspect` → `cosign verify` → pull by digest →
   `podman image inspect` → temporary start → HTTP check. Cover wrong digest,
   missing signature, wrong architecture, forbidden root user, and failed
   startup.
2. **Network preflight:** DNS A/AAAA → route → TLS/SNI → HTTP `Host` → status
   and control response. Cover a broken AAAA, wrong certificate, wrong virtual
   host, 404/500, and finite timeout.
3. **Exposure audit:** compare runtime port/network descriptions with `ss`,
   `ip`, and firewall rules. Reject unexpected `0.0.0.0`, IPv6, administrative
   ports, or networks; test rootless and rootful runtimes separately.
4. **Safe image cleanup:** preserve current and rollback digests, include
   stopped containers when checking references, show deletion candidates in
   `--plan`, and make reruns safe. Do not begin with broad `prune`.

Do not add a container runtime, OCI model, built-in DNS/TLS or firewall module,
signature policy, Compose model, or Kubernetes API. If two independent stands
do not expose the same ordering or proof problem, keep the solution in external
commands.

### 5. Polish tutorial and manuals

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
  check, `❌ ` marks a concrete failure or prohibited option, and `👀 ` marks
  something the administrator must check personally. The adjacent prose must
  remain understandable without emojis.

## After `v0.2.0`: prove restoration

First polish the existing scenario set rather than adding another built-in
module. In two independent scenarios, show the complete path: preflight,
transfer, failure, rerun, publication of a completed tree, and restoration to a
temporary destination with content verification.

💡 Problem solved by yesh: a backup command returning code `0` is not presented
  as proof that the copy can actually be restored.

Add a new capability only if these scenarios expose a repeated problem that
cannot be expressed clearly with visible external commands. It must have
positive, negative, and `--plan` cases, together with an explicit description
of the new state model it introduces.

---

## Separate future milestone: `yesh navigator`

The navigator remains an experimental companion milestone. Before adding new
screens, finish extracting it into a separate `yesh-navigator` executable. The
normal `yesh` binary should not require the TUI dependency just to run a script.

The extraction is an architectural refactor, not a file move:

1. freeze the current navigator behaviour and its parser/plan tests;
2. as an interim boundary, build the navigator only with the `navigator` build
   tag and publish it as a separate optional `yesh-navigator` artifact; the
   default `yesh` build must not include the TUI;
3. move the shared lexer, parser, AST, and plan renderer into a private package
   such as `internal/yeshcore` without duplicating grammar or semantics;
4. make `cmd/yesh` depend on that package for ordinary script execution;
5. build `cmd/yesh-navigator` against the same package and keep the navigator
   limited to composing text, parsing it, showing the plan, and saving a new
   file;
6. remove the `yesh navigator` subcommand from the main binary; the navigator is
   an optional companion tool, not part of the core command contract.

The interim step is complete when the normal binary builds without the TUI
implementation and the tagged artifact passes its navigator tests. The final
extraction is complete only when the optional command uses a shared parser and plan
output from `internal/yeshcore`, and the
positive, negative, and non-executing plan tests pass for both entry points.

The English full-screen interface should follow `nmtui`: a main activity menu,
a separate parameter form, a review of the generated yesh script, and its
`--plan` output. Do not show placeholders; start with only `Create a backup`
and `Quit`.

Use the model `MainForm → BackupForm → ReviewForm`, with field focus and
`Tab`/arrow/`Enter`/`Esc` navigation. Show selected buttons as `[ OK ]` and
`[ Cancel ]`, with a visible contrast change for the active widget. Use
`tcell/v2`, which is compatible with Go 1.21; increasing the Go version is not
required. The navigator must not run external commands.

The form should use distinct widget roles, following `newt`: `entry` for
text, `label` for explanations, `listbox`/`actlistbox` for choices, `checkbox`
for flags, and `button`/`actbutton` for actions. `Tab` must move between
widgets and buttons; the interface must not reduce every control to a line in
one undifferentiated list.

The first prototype generates a backup script, reparses it with the existing
parser, and saves it only to a new file with mode `0600`. It must not execute
commands or overwrite an existing file.

The backup form must distinguish a local path from an `rsync` remote source.
For a remote source it must not perform a local `exists` check.

### Navigator as a teaching assistant

The navigator is for a person seeing yesh for the first time. It should help
them understand and assemble a working scenario, not present a collection of
unrelated fields or generate unexplained code.

### Navigator design principles

The navigator's primary result is a saved, validated script, not an executed
action. It must never run the script it assembles: it helps identify intent,
compose a complex scenario, validate the text with the parser, show the plan,
and explicitly save a new file.

The parser is the single source of truth for yesh syntax and semantic
boundaries. The navigator must not reimplement validation of service names,
durations, managers, or allowed constructs: it builds text, passes it to the
parser, and displays the resulting error or AST/plan.

Validation is one-pass. The navigator must not silently repair text, guess the
parser's intent, or enter a `generate → parse → repair → parse` loop. After an
error it displays the reason and waits for the user's next explicit edit.

1. Discover intent and context before showing all technical parameters.
2. Ask only the next necessary question. Every question must have a clear
   reason, an example answer, and a visible effect on the resulting script.
3. Reveal complexity progressively: choose the action and responsibility
   boundary first, then paths, checks, locking, and other details.
4. Keep execution visible. Show what yesh will check, which external `argv`
   it will pass, and what remains the responsibility of the external command,
   filesystem, or remote host.
5. Do not silently guess or repair dangerous values. Stop contradictory or
   risky choices and explain them in plain language.
6. Validate every result with the existing parser and show it through
   `--plan`; review must never run external commands.
7. Save only on explicit confirmation, to a new file, without overwriting an
   existing file, and report failures clearly.
8. Every screen must have working keyboard navigation, visible focus, clear
   buttons, and a way back. Never show dead controls or leave the user stuck.

### Wizard rules derived from TUI practice

- One wizard solves one task. Backup creation and service diagnosis are separate
  branches, not one universal flow.
- One screen carries one idea: context, parameters, review, or saving. Do not
  hide a new independent decision inside an existing screen.
- Reveal complexity progressively. Start with intent and responsibility
  boundaries, then show only the parameters that matter; keep advanced detail
  out of the first view.
- Keep branching steps stable and legible. Show `Step X of Y`, and name commit
  buttons after the action: `Continue`, `Review`, or `Save`, not an anonymous
  `Next`.
- A smart suggestion must be visible, explained, and editable. A default is an
  interface recommendation, not a language rule or hidden configuration.
- Put errors near their cause and explain the next action. Never show a red
  message without identifying which field needs attention.
- Only interactive controls belong in the Tab order, in reading order. Titles,
  help text, and scrollbars do not receive focus.
- The wizard page itself should not require scrolling; only large lists inside
  it may scroll.

References: [Microsoft wizard guidance](https://learn.microsoft.com/en-us/windows/win32/uxguide/win-wizards),
[IBM progressive disclosure](https://www.ibm.com/docs/en/technical-content?topic=practices-progressive-disclosure),
[nmtui documentation](https://networkmanager.pages.freedesktop.org/NetworkManager/NetworkManager/nmtui.html),
and [W3C keyboard interface guidance](https://www.w3.org/WAI/ARIA/apg/practices/keyboard-interface/).

Practical reference patterns:

- `nmtui`: activity menu → one task → list or form;
- `ansible-navigator`: create and review automation with separate interface
  modes and results that can be inspected independently;
- configuration TUIs: wizard → command preview → explicit save with a clear
  result message.

The target yesh navigator sequence is:

```text
choose a scenario → understand intent → ask only necessary questions
→ compose yesh → parser → show --plan → save the file
```

The navigator never runs the composed file, silently guesses, or reduces a
complex task to a collection of unrelated technical fields.

### Navigator pause point

The working prototype now uses `tcell/v2`: main activity menu, backup script
composition, systemd service discovery with filtering, completion and scrolling,
`system`/`user` selection, resource snapshot/measurement/summary choices, parser
validation, and saving a new file without execution.

After the pause, the shared safe `--plan` output is now connected to the runtime:
the navigator passes it the parsed AST and receives the plan through a buffer,
without starting another process or duplicating semantics. Both backup and
service scenarios use this path. A local nginx configuration backup was also
exercised: the changed file was transferred, the repeat run was empty, and
extra target files were not deleted.

### Plan for the next few days

1. Finish manual TUI verification: keyboard navigation, scrolling, completion,
   save errors, short terminals, and terminal resize.
2. Test the complete path `intent → questions → parser → --plan → Save` with
   positive, invalid, and interrupted cases.
3. Do not present a simple `findmnt --target` check as proof of the intended
   backup disk: a directory on the root filesystem passes it too. Connect
   UUID/PARTUUID, SSH, and remote-path checks only as explicitly selected
   advanced checks; never auto-select a device and add no new built-ins for
   this integration.
4. Test `View measurements`: no history, corrupt journal, an `InvocationID`
   change, scrolling, and the neutral `=` comparison.
5. Add a detailed single-point view and RSS/CPU indicators only if existing
   data provides a clear, verifiable basis.
6. Then clean up code, tests, and TODO; add no new navigator branches before
   these checks are complete.

### Services and resource observation

Add a separate `Inspect a service` branch. First show the services visible to
the selected manager (`system` or `user`), then explain and prepare for the
selected service:

1. a one-shot `service resources SERVICE` snapshot;
2. a finite `measure service SERVICE ...` run;
3. accumulated history through `summarize service`.

The list is read-only and must be labelled honestly: it contains services
currently loaded by the manager, not every installed unit file. When the user
types a service name, filter by prefix and offer completion: `Tab` or an arrow
selects a suggestion, and the script receives the full name such as
`nginx.service`. Do not add hidden `start`, `stop`, `restart`, `enable`, or
continuous polling to this screen. The navigator may perform bounded read-only
discovery and status reads, but it must not change services or present a finite
snapshot as a permanent monitor.

The flow should adapt to the task:

```text
user intent → context → next necessary question → assumption checks
→ explanation → plan → yesh → parser → --plan
```

Every question must explain why it matters, show an example, and display the
corresponding yesh fragment. Do not ask questions unrelated to the selected
task. Stop contradictory or unsafe choices and explain the reason.

### Short measurement-history path

For the ordinary case, the navigator should use one short user-owned path:

```text
~/.local/yesh/<service>.jsonl
```

For example, the history for `nginx.service` is stored in
`~/.local/yesh/nginx.service.jsonl`. `system`/`user` are not put into the path:
they are the systemd manager context and a validated field of every JSON record.
If one file is used for different managers or services, yesh refuses to mix the
histories silently. A different path is used only through explicit `save to`.

The future `View measurements` action should not be permanently shown in the
main menu. After the user chooses a manager and service, the navigator checks
only the exact canonical path and adds this action when it finds non-empty
history. The navigator does not scan it implicitly at startup. A corrupt record,
incomplete tail, or foreign history is shown as an error rather than as missing
history; the navigator never repairs it automatically.

The `View measurements` window must be useful while remaining read-only: show
the service and manager, path, point count, and time range at the top; below,
show a scrollable list with each point's finish time, duration, completeness,
`InvocationID`/cgroup, and memory change. An incomplete point must show its
reason (`identity_changed`, a disappeared process, or an unavailable metric).
Raw JSONL lines are not the primary view; an optional single-record view may
show them without changing the file.

For adjacent comparable points, show compact indicators beside metrics: `↑` for
an increase, `↓` for a decrease, `=` for no material change, and `?` when the
comparison is unavailable. Compare only points for the same service, cgroup,
and `InvocationID`; do not draw an arrow across a restart boundary. Show the
actual delta and source values beside the marker rather than relying on color
or an icon alone.

The word “anomalous” must not be inferred from visual impression. It requires a
separate explicit rule, such as a configured threshold or a selected baseline
period. Until that rule exists, the navigator may say “sharp increase” as an
observed change, but must not claim a cause.

The result should show three layers:

1. what the user wanted to do;
2. what yesh will check and in which order;
3. which external `argv` will run and what remains the responsibility of
   `rsync`, the filesystem, or the remote host.

Use `ansible-navigator` as an external reference for a TUI that helps create,
review, run, and troubleshoot automation. yesh additionally needs teaching
explanations and must reparse the generated text with its existing parser.

### Navigator palette

Use `newtDefaultColorPalette`, the palette family used by `nmtui`, as the
reference. `nmtui` exposes color roles through `NEWT_COLORS`; preserve the
roles exactly while allowing the shades to remain a theme choice.

The base `nmtui-blue` theme is:

- root: white on blue;
- window and border: black on light gray;
- title: red on light gray;
- label: blue on light gray;
- normal list item: black on light gray;
- active list item: light gray on blue;
- normal button: light gray on red;
- active button: red on light gray;
- entry: light gray on blue;
- disabled entry: blue on light gray.

Turbo Vision remains a reference for the general idea of dialogs, focus, and
buttons. The first implementation should take its colors from `nmtui` so the
interface remains legible and does not visually merge together.

## Longer-term direction after polishing

The next major value of yesh is not another shell and not a replacement for
mature Linux tools, but a verifiable operational path around them. The backup
and restore path already belongs to the completed body of work; the longer-term
search should look for other scenarios where the glue between commands is
especially painful.

Keep these items in the longer-term plan:

- do not expand the completed backup body without a new confirmed scenario;
- for new operational scenarios, report not only that a command succeeded, but
  also what yesh checked and what remains the responsibility of the external
  command, filesystem, or remote host;
- keep the navigator as a newcomer entry point for two bounded workflows:
  backup and service inspection. In both, the form only builds text, while the
  parser validates it before plan review and saving;
- later separate the measurement target from the systemd adapter: consider
  reading an arbitrary cgroup v2, a process tree, and, separately, a remote
  target over SSH. Account for Alpine and other OpenRC systems where systemd
  and journald may be absent; keep `service` as the first and strictest
  adapter;
- collect feedback from real scenarios and add built-in help only where it
  reduces risk or makes the result easier to explain.

This branch requires positive, negative, and `--plan` cases. A successful exit
code or a newly created directory is not proof of the intended operational
result.

This direction does not change yesh's boundary: `rsync`, `rsnapshot`, SSH, and
systemd remain external tools. yesh is responsible for visible ordering,
checks, locking, publication, and understandable refusal.

### Longer-term search: scenarios where Bash is especially painful

After `rsync` and `rsnapshot`, collect real examples where each Unix utility is
mature on its own, but the connection between them is held together by fragile
Bash. The target is not a reason to replace the shell wholesale, but a repeated
pain around ordering, partial failure, reruns, and proof of the result.

Candidates for investigation:

1. version publication: `rsync`/`scp` → configuration validation → atomic
   replacement → `systemctl restart` → readiness check → journal and return to
   the previous version;
2. service or container maintenance: preflight → lock → stop through an
   external command → maintenance → start → readiness check;
3. configuration publication that refuses before changing the active file;
4. cron or systemd-timer jobs with locking, a finite deadline, result checks,
   and an understandable failure notification;
5. maintenance of a minimal Alpine/OpenRC host: transfer configuration or a
   built artifact → external validation → publication → `rc-service` → process,
   cgroup, and available-log checks. Test the case without systemd and
   journald separately, where the result must be supported by OpenRC, `/proc`,
   cgroup, and ordinary log files;
6. updating one external Compose stack: check space and current containers →
   pull selected images → recreate changed containers → verify readiness and the
   previous version → defer cleanup until success;
7. a constrained external `ssh` scenario for one host with explicit `argv`, a
   result for each step, and termination after a partial failure.

Start with version publication and service-state verification after the change.
Do not start the other directions in parallel or turn them into new built-in
modules until two independent scenarios show the same problem.

#### Boundary: do not build a second Ansible

yesh does not add inventory, roles, desired state, group variables, mass host
management, a resident agent, a central state store, or automatic drift
correction. An external SSH tool or fleet-management system may pass a scenario
to several hosts; yesh remains a clear scenario for one host or one local
operation. If the solution requires describing a fleet and its state, it is no
longer a yesh problem.

Alpine must not become a separate language model or a set of hidden wrappers
around `apk`, OpenRC, and BusyBox. The useful scenario is a verifiable sequence
around external commands on a minimal Linux host. Do not add a package manager,
an OpenRC replacement, a general logging layer, or a promise of identical
behaviour across all Alpine images and hosts.

### Containerized environments without a shell

[Containerized operational pain map](underthehood/container-operations-findings.md)
collects the sources, primary constraints, and yesh applicability boundaries.

Alpine is only the first practical stand. The next hypothesis is that yesh is
useful in a minimal containerized environment where `bash` is absent, while a
distroless image may have no shell, package manager, or ordinary utilities at
all. Do not restore a shell to such an image and do not put yesh in every
application image. First test a separate one-shot operational image containing
yesh, exactly the required external programs, and explicitly mounted volumes or
network access.

This work also clarifies yesh's broader niche: yesh describes reviewable
operational procedures. The procedure may run on a host or inside a container.
The next serious acceptance gate is interruption and rerun: after an abrupt
stop, the next run must establish whether the lock was released, what result
was published, and what remains safe to retry.

The absence of a shell is not, by itself, a yesh problem. If a scenario needs
`tar`, `cp`, a diagnostic utility, or a database client, the image must provide
it explicitly and that tool remains responsible for its work. yesh must not
replace missing programs with hidden built-ins.

Investigate these scenarios:

1. **One-shot operational image.** An Alpine image contains yesh, the required
   external utility, and the application's command: for example, a database
   migration, import/export, volume check, or configuration publication. yesh
   owns preflight, ordering, locking, a finite deadline, and the exit code, but
   does not replace the migrator, database, or container runtime.
2. **A volume operation from a separate container.** A one-shot container with
   an attached volume performs only explicitly requested reads, checks, or
   publication. Source, destination, permissions, and mount boundary must be
   visible in the plan; do not rely on interactive `exec` and hand-written Bash.
3. **A verified application entrypoint.** Only after a separate experiment,
   consider yesh as an explicit `ENTRYPOINT`: check environment, directories,
   permissions, and volumes, then hand control to the application with exact
   `argv`. Do not turn this into a supervisor, restarter, or `tini` replacement;
   separately prove signal delivery and child-process termination.
4. **Diagnosis without entering the image.** yesh runs outside and invokes
   `docker`, `podman`, or `kubectl` with explicit `argv`, or runs in a separate
   diagnostic container. It produces a finite report about the process,
   environment, volumes, network check, and logs, but does not become a debug
   system, container agent, or orchestrator.
5. **Image-contract testing.** An external runtime builds and starts the image,
   while yesh checks exit code, signal, readiness, expected logs, user,
   permissions, and cleanup after failure. This tests the image and scenario;
   it is not a new Dockerfile language or a replacement for `docker`/`kubectl`.

First choose a one-shot Alpine operation without `bash`: a separate operational
image, yesh, one mature external command, and a temporary mounted volume. Then
separately test whether the same approach can work in a distroless image with
only a static yesh binary and the explicitly required application command. If
it requires adding `sh`, `ls`, `tar`, or a package manager, treat that as a
negative result for the hypothesis, not as a reason to enlarge the image.

The first concrete laboratory stand is an Alpine image with a static yesh binary
and external `sqlite3`, a temporary mounted volume, an initial database, and one
migration. The scenario must demonstrate successful application, failure without
damaging the initial state, rerun behaviour, and a non-mutating `--plan`. SQLite
is only a small laboratory stand for testing the model; the eventual question is
whether the same ordering transfers to a one-shot PostgreSQL `Job`, not whether
yesh should gain built-in database support.

Result of the first stand:

- [x] built a multi-stage Alpine image with a static Linux/amd64 yesh binary and
  external `sqlite3`;
- [x] positive migration passed on a named Docker volume;
- [x] rerun reported that the migration was already applied;
- [x] `--plan` showed the lock, external `argv`, redirection, and both check
  branches without running the migration;
- [x] the negative SQL migration failed and left no partial column after adding
  `.bail on`;
- [x] the image ran with `--read-only`, `--network none`, no added capabilities,
  and an explicit Docker `ENTRYPOINT`/`CMD`.

This stand proves only a one-shot Alpine operation with a named Docker volume on
Linux/amd64. A WSL2 bind mount of a temporary directory failed with a permission
error in this Docker context; this is a separate Docker/WSL2 boundary, not proven
yesh semantics. Kubernetes `Job`, CSI/NFS, `fsGroup`, distributed locking, and a
real PostgreSQL setup remain untested.

The Linux `with lock` implementation uses an advisory `flock` on an open file
descriptor. The kernel releases that lock when the process exits, including
`SIGKILL`; a remaining lock-path file is only a pathname and does not prove that
the lock is held. Automated tests cover a body error and a competing holder.
Manual interruption checks are now complete:

- [x] kill a yesh process while it owns the lock on the Alpine 3.24.3 stand;
  a second process acquired the lock;
- [x] run the same check in the WSL2 Docker stand with `docker kill` and the
  same named volume; a second container acquired the lock.

These checks prove lock release and reacquisition. They do not prove that an
arbitrary interrupted migration or transfer leaves valid data; each operation
still needs its own partial-result and rerun test.

Signal handling, child-process ownership, and PID 1 behaviour are established
Linux and container responsibilities. yesh does not need to invent another
init mechanism. The engineering task is narrower and necessary: verify that
yesh preserves these existing guarantees when it runs an operational procedure.

The current POSIX runtime turns `SIGTERM`, `SIGINT`, and `SIGHUP` into context
cancellation, sends `SIGTERM` to the command's process group, waits two seconds,
escalates to `SIGKILL`, and waits for direct children. Existing POSIX tests cover
process groups, cancellation, timeouts, and child reaping. They do not yet prove
the container contract with yesh as PID 1.

Add a Docker test on the same scenario. Test yesh's signal and child-process
behaviour directly, and delegate the PID 1 role to the existing `tini` package
or another mature init:

- run the yesh procedure under `tini` with the exec-form entrypoint;
- start a long-running external command and a descendant that ignores `SIGTERM`;
- send `docker stop` and verify signal delivery, process-group cleanup, exit
  within the grace period, and no remaining descendant;
- repeat with a partial-result scenario and verify that the next run sees the
  previous published result, not a false success.

The result is a boundary statement, not a novelty claim. yesh must not become an
init, supervisor, or `tini` replacement. `tini` or another mature init owns the
PID 1 role. yesh owns the predictable cancellation of the external commands it
starts, process-group cleanup, child waits, and the visible failure result.

Documentation for the stand:

- [x] add a [practical explanation of the stand](underthehood/container-sqlite-lab.md);
- [x] state the responsibility boundary explicitly: yesh owns ordering,
  locking, checks, `--plan`, and the exit code; `sqlite3` owns the database,
  SQL, and transaction; Docker owns the image, process, and volume;
- [x] show a short Bash equivalent beside it for comparison only;
- [x] document build and run commands, including a named volume and
  `--network host` only as a WSL2 DNS workaround during the build;
- [x] show the positive run, rerun, rollback failure, and `--plan`, and state
  separately that the local lock is not distributed;
- [x] do not present this stand as validation of Kubernetes, PostgreSQL, CSI/NFS,
  or universal shell-less image support.

The most concrete candidates for the first comparison are:

- **a database migration as a one-shot Job**: wait for database availability,
  validate configuration and secrets, run the external migrator, preserve its
  exit code and log, and allow the next step only after success;
- **volume preparation**: prove that the expected destination is mounted, then
  use external `chown`/`chmod`/file checks to prepare it for a non-root process
  without changing the wrong path after a failure;
- **evidence collection from a shell-less image**: explicitly invoke available
  read-only tools and produce a finite result about the process, environment,
  permissions, volumes, and logs without installing tools into the application
  image.

For migrations, yesh must not present a local `with lock` as distributed
protection: with multiple nodes, the database, migrator, or external `Job` owns
the decision that only one migration runs. Volume preparation requires separate
checks for permissions, ownership, mount point, and rootless operation. Diagnosis
must prove that the report belongs to the intended process and namespace, not to
the diagnostic container itself.

Boundary: do not add a container runtime, Kubernetes API, process supervisor,
ephemeral-container system, package manager, interactive shell, or a language
for describing an entire CI pipeline. yesh may be one explicit step inside a
container or an external scenario around one; it does not manage a cluster and
does not become a second Ansible.

For every candidate, prepare a positive scenario, partial failure and rerun,
negative scenario with no data change, `--plan` with expanded external `argv`,
a short Bash equivalent, and the boundary of proven behaviour. If a visible
external command is already clear enough, do not add a new language construct.

### Longer-term measurement target model

The general future measurement shape is:

```text
target → confirmed identity → finite interval
→ measurements → completeness → explainable result
```

Possible targets, in increasing order of complexity:

1. a systemd service with `InvocationID`, cgroup, and journal;
2. an arbitrary cgroup v2, including a container group;
3. a process tree identified by PID;
4. a remote target through an explicitly selected SSH path.

On Alpine, `/proc` and `/sys/fs/cgroup` may be the primary sources, while
service state and journal data must remain optional adapters. If cgroup v2, a
required controller, or permissions are unavailable, the result must state
the limitation rather than replacing it with zero.

For a PID, verify start time and process membership: a PID number alone is not
enough because processes exit and PIDs are reused. For a cgroup, retain the
path, available controllers, and subtree boundary. A single discovered process
must not be treated as proof of the whole target.

Each new target type requires separate positive, negative, and `--plan` cases.
Do not add a generalized measurement construct until at least two independent
practical scenarios justify it.

### Constrained OS stand: Alpine/OpenRC

Before implementing an OpenRC adapter, proofread and validate the [design note
on resource-constrained operating systems](underthehood/resource-constrained-os.md).
The acceptance criteria include:

- cgroup v2 is mounted before services start; already running processes must
  not be treated as proof that each service has its own cgroup;
- distinguish `rc-update add` (runlevel change) from an actual start through
  `rc-service` or `openrc`;
- do not treat `rc-status`, a PID file, or `zap` as complete proof of process
  state;
- for `supervise-daemon`, retain child PID, start time, respawns, and process
  epoch changes;
- keep OpenRC state, cgroup, processes, and syslog/file logs as separate layers;
- test system and user services separately, including permissions and
  `XDG_RUNTIME_DIR`;
- report missing journald, PSI, a controller, or cgroup delegation as
  `unavailable`/an incomplete observation.
- [x] document checking `uname -m`/`apk --print-arch` before choosing an Alpine
  executable;
- [ ] verify building and running yesh on ARM64, ARMv7, or MIPS; cross-compiling
  alone is not such a test.

Alpine 3.24.3 with OpenRC 0.63.2 has been checked as a separate lower-level
stand: machine-readable `rc-status`, separate supervisor and child PIDs, one
shared cgroup, and an increased respawn count after terminating the child were
confirmed. This proves only the selected kernel, Alpine, and OpenRC
configuration. A positive run of the built yesh binary for the ordinary `sshd`
service confirmed state, platform, supervisor, cgroup, process snapshot, and
finite JSONL output; a missing-service request failed without false success.
The dedicated `supervise-daemon` run confirmed a new child-process epoch: the
measurement became incomplete after the restart without mixing the two runs.
OpenRC user-service validation remains open.

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

### Separate future initramfs profile

The practical value of yesh here is a pre-reboot check, not its own image
builder or a “boot guarantee”. The first slice should take an explicit
kernel-version/initramfs-image pair, check `root=`/UUID/PARTUUID, filesystem
type, expected modules, and firmware through external `lsinitrd`/`lsinitcpio`/
`lsinitramfs`, `modinfo`, `findmnt`, and `lsblk`.

The report must distinguish proven, unproven, and uninspectable facts: a
built-in driver does not need a `.ko`, a `.ko` in the image does not prove
device binding, host-only limits portability, and LUKS/LVM/RAID form a separate
chain to the root filesystem. A multi-segment image with early microcode, a
UKI, or an incomplete inspection must not be reported as an image missing a
module.

Until a positive and negative temporary-VM scenario exists, do not add writes
to `/boot`, `dracut --force`, `mkinitcpio`, `modprobe`, reboot, or a new built-in
type. `--plan` must perform none of those operations.

Build the stand in layers:

1. temporary kernel/initramfs/root-disk artifacts and content inspection without
   booting;
2. a separate QEMU VM with `-kernel`, `-initrd`, a temporary disk,
   `console=ttyS0`, `-nographic`, a finite timeout, and `-no-reboot`;
3. a positive boot and failures for a wrong UUID, missing driver or filesystem,
   incomplete image, and host-only output on different virtual hardware;
4. a separate UKI/OVMF level with test Secure Boot keys.

The stand must not attach real host devices. A successful QEMU boot proves only
the selected pair on virtual hardware; real firmware, LUKS/LVM/RAID, and
physical controllers require separate checks.

Preserve a diagnostic artifact from a failed early boot (`rdsosreport`, serial
console log, or equivalent) when one is produced. Compare a normal and a
fallback image separately: a fallback may contain more drivers, but its
presence does not prove that the main image is bootable. Keep “UUID not found”
distinct from a missing driver, filesystem, image, or `/boot` space problem.

## Next working step

Keep the backup boundary stable and use the completed `rsync`/`rsnapshot`
examples as the acceptance reference. Do not add network capabilities or
metrics yet; the next changes should be small documentation or test repairs
against a real failure.

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
