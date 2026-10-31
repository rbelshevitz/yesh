# Current yesh roadmap

This is the short working roadmap. Detailed decisions, completed checks, and
historical planning remain in the [archived roadmap](TODO-history.md).

## Current position

yesh is a readable language for sequential Linux automation. Linux commands
remain visible, while yesh makes arguments, ordering, preconditions, and
failure behavior easier to review.

Confirmed today:

- assignments, strings, lists, `if`, `for`, commands, pipelines,
  redirections, and `--plan`;
- direct Go-runtime process execution without passing the script to Bash;
- command and pipeline failure handling, time bounds, cancellation, and child
  process cleanup;
- `exists`, regular expressions, `exit code`, locking, and same-filesystem
  publication;
- practical workflows around `rsync`, `rsnapshot`, `tar`, `pv`, SSH/SCP,
  systemd, journals, storage, and `/proc`;
- every `examples/*.yesh` file is parsed by `TestAllExamplesParse`;
- Linux CI, a separate Debian host, and an ext4 loopback fixture pass.

These results do not promise universal OS support, power-loss durability,
isolation of untrusted code, or success of an external utility in every host
environment.

## Immediate priorities

### README as the project entry point

- keep purpose, audience, one useful example, quick start, and links to detailed
  documentation on the first screen;
- distinguish the portable yesh layer, ordinary Unix utilities, and Linux-only
  features;
- verify quick-start commands in a clean environment;
- keep the image and label environment-dependent examples honestly.

Ready when a new reader understands the value of yesh and runs the first
self-contained example before reading the architecture.

### Backup and rsnapshot

- show the actual `rsnapshot -t` output before a changing operation;
- surface known deletion flags and check `snapshot_root`, mounts, lockfile,
  ownership, and `no_create_root`;
- prepare a production-like RWE on a separate filesystem with a known UUID;
- test SSH sources, a full filesystem, an unmounted destination, partial
  transfer, and rerun behavior.

`rsync` and `rsnapshot` remain external engines. yesh does not mirror their
configuration grammars or rotation implementations.

### Storage reliability

- define an `rsync --fsync` and directory-fsync profile;
- inspect filesystem type and effective mount flags before changing data;
- never describe an atomic rename as power-loss durability.

## Later priorities

- networked rsync failure matrix and safe retry behavior;
- controlled `PATH` and environment policy for privileged execution;
- durable job state, job identity, and resume rules;
- typed operations only after two independent scenarios justify them;
- read-only cron and package queries without hidden installation or mutation;
- independent review of the tutorial by a Linux administrator.

## Boundaries

- yesh is not an interactive shell or a Bash-compatible language;
- do not reimplement `rsync`, `rsnapshot`, `systemctl`, `mount`, `sed`, `awk`,
  or a package manager;
- do not add `cd`, `export`, command substitution, TTY/PTY, or other shell
  features without a demonstrated need;
- macOS/Darwin work is deferred;
- root, real devices, systemd, SSH, and production paths require a separate
  environment.

## Evidence and documentation

- [Practical Examples](practical-examples.md)
- [example map](example-map.ru.md)
- [execution safety](../SAFETY.md)
- [syntax tutorial](syntax-guide.md)
- [Russian roadmap](TODO.ru.md)
- [archived roadmap](TODO-history.md)
