# Bash corpus for yesh

The corpus evaluates yesh against real administrative tasks; it does not vendor
third-party code. For each case we record the task, write an independent yesh
adaptation, and compare its execution plan with `--plan`.

## Sources with an explicit license

| Source | License | Task families |
| --- | --- | --- |
| [dataCore/bash-scripts-collection](https://github.com/dataCore/bash-scripts-collection) | MIT (stated by the repository) | SSH hardening, Docker backup/restore, mountpoint checks, Debian updates |
| [wesleyscholl/bash-scripts](https://github.com/wesleyscholl/bash-scripts) | MIT ([LICENSE](https://github.com/wesleyscholl/bash-scripts/blob/main/LICENSE)) | disk monitoring, backup/rsync, log rotation, SSL and service checks |
| [pedroetb/rsync-incremental-backup](https://github.com/pedroetb/rsync-incremental-backup) | MIT (stated in README/repository) | incremental rsync, retention, include/exclude, remote SSH |
| [errnair/sysadmin-shell-scripts](https://github.com/errnair/sysadmin-shell-scripts) | MIT (stated in README and `LICENSE`) | system stats, config backup, user audit, syslog and JSON/CSV output |
| [magnetikonline/rsync-backup-rotation](https://github.com/magnetikonline/rsync-backup-rotation) | MIT (stated on the repository page) | incremental rsync directory rotation and `--link-dest` |

Pinned source points (29 October 2026):

- `pedroetb/rsync-incremental-backup` — `9909d78ac29bd128e90a3c39aed94643c6bf5fe7`, `rsync-incremental-backup-local`.
- `dataCore/bash-scripts-collection` — `d08fb8074ab71d562360ac7222c72d3ebae64643`, `backup-docker-all.sh` and mountpoint checks.
- `wesleyscholl/bash-scripts` — `468a774223c41e3816ea30f059de9b9321b81190`, disk-monitoring and backup scripts.
- `errnair/sysadmin-shell-scripts` — `621c2129e65a92103a4bf2e0905f72746a11e1f2`, `server_management/system_stats.sh` and `utilities/dirbackup.sh`.

Use behavioral requirements from these projects and implement adaptations
independently. If code is imported verbatim, preserve its copyright/license
notice and record provenance. Do not use repositories without an explicit
license as code sources; see [GitHub's licensing guidance](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/licensing-a-repository).

Every selected case needs observable inputs and side effects, positive and
negative tests, and a concrete `--plan` comparison. Add a new yesh builtin only
when the same problem appears in at least two independent scripts.

This is currently a candidate catalog, not vendored code. Next, select 20–30
scripts, pin each source URL and commit, then add adaptations under `examples/`
with tests.
