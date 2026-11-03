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
- The storage checks are sufficient for the backup boundary: yesh verifies the
  mount, expected source, and filesystem with `findmnt`, space with `df`/
  `free_space`, and path properties with `stat`, then stops before `rsync` or
  `rsnapshot` when the destination is not proven. Mounting and unmounting stay
  outside `v0.2.0`.
- `storage-preflight.yesh` was checked with a placeholder UUID: `--plan` only
  shows deferred checks, while a real run stops during device resolution and
  does not attempt to mount anything. A positive run with a real UUID-backed
  partition remains an external Linux-stand check.
- On a separate Linux stand, `backup-mounted-rwe.yesh` passed positively:
  loopback/ext4 with a real UUID, backup write, `--plan`, wrong UUID,
  read-only, unmounted target, and low-space cases. All negative checks left
  the target unchanged.
- On a separate Debian node, SSH/SCP and real `rsync` passed in both directions.
  The loopback/ext4 `rsnapshot` stand passed with `no_create_root`, two
  rotations, hard links, restore, `--plan`, negative checks, and cleanup; both
  normal mode and `sync_first=1` (`sync` → rotation) were verified separately.
- A second physical partition is not required for this acceptance: the
  temporary loopback/ext4 image is sufficient. A real destination such as
  `/var/cache/rsnapshot` still requires its own mounted filesystem; a directory
  on the root filesystem is not an accepted backup destination.
- When a separate partition becomes available, repeat the `rsnapshot` check on
  the real mount: verify the UUID, `no_create_root=1`, refusal while unmounted,
  and a successful rerun after mounting it again.
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

Repeat and review the `rsync`/`rsnapshot` matrix, separately verify restoration
and negative cases, then update the tutorial and practical examples. Release
`v0.2.0` is published; do not add network capabilities or metrics yet.

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
