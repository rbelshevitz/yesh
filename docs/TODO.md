# Current yesh roadmap

This is the short working map. Historical decisions and completed checks are
kept in the [archive](TODO-history.md); this file is not a promise to implement
every item listed below.

## Current position

yesh is a readable language for sequential Linux automation. External Linux
commands remain visible; yesh makes arguments, ordering, preconditions,
publication boundaries, and failure behavior easier to review.

Confirmed:

- assignments, strings, lists, `if`, `for`, commands, pipelines, redirects,
  `--plan`, `exists`, regular expressions, and explicit `exit code`;
- direct Go-runtime process execution, bounded commands, cancellation, and
  child-process cleanup;
- advisory locks and same-filesystem `publish` without copy fallback;
- practical scenarios around `tar`, `gzip`, `pv`, `rsync`, `rsnapshot`, SSH/SCP,
  systemd, journals, storage, `/proc`, and read-only network observation;
- Linux CI, WSL2, separate Debian fixtures, Rocky Linux checks, and loopback
  ext4 scenarios;
- more than 200 automated tests and four fuzz targets.

The first public development release is `v0.1.0` for Linux amd64. Its GitHub
draft contains the binary and checksums. ARM64 is not part of the release until
the binary has been run on real ARM64 hardware or an ARM64 virtual machine.

## Next after the pause

1. Publish the existing `v0.1.0` GitHub draft.
2. Prepare a production-like rsnapshot/storage scenario on a separate
   filesystem, keeping durability limits explicit.
3. Run an ARM64 smoke test and decide whether to add ARM64 to a later release.
4. Design a separate external-command policy with explicit `allowed`, `denied`,
   and `unknown` outcomes; `allowed` means policy match, not execution success.
5. Consider controlled `PATH` and environment policy for privileged execution.
6. Consider durable job identity and resume rules only after a concrete use case.

The networked `rsync` preflight, partial-transfer, failure, and retry scenarios
have already been exercised on the disposable Debian stand. Do not add a new
network language layer or repeat those checks without a specific new failure.

## Boundaries

- yesh is not an interactive shell, Bash replacement, sandbox, or backup engine;
- rejecting `bash -c` and similar wrappers is a language rule, not an isolation
  boundary;
- `rsync`, `rsnapshot`, `systemctl`, `mount`, `sed`, `awk`, SSH, and package
  managers remain external tools with their native semantics;
- do not add `cd`, `export`, command substitution, TTY/PTY, or broad shell
  compatibility without two independent practical scenarios;
- do not claim power-loss durability, universal OS support, or success of an
  external command in every environment;
- macOS/Darwin work remains deferred.

## Evidence and documentation

- [Practical Examples](practical-examples.md)
- [example map](example-map.ru.md)
- [execution safety](../SAFETY.md)
- [syntax tutorial](syntax-guide.md)
- [Russian roadmap](TODO.ru.md)
- [archived decisions and evidence](TODO-history.md)
