# Practical stand: one-shot Alpine migration without Bash

This stand does not add SQLite support and does not replace a container
runtime. It tests a narrower idea: yesh can be an explicit executable scenario
in a one-shot minimal image without Bash glue.

In short, yesh is neither the migrator nor the container manager here. It joins
preflight checks, a local lock, an external command, and result verification.
The database, SQL migration, and transaction rules belong to `sqlite3` and the
application; the image, volume, and process limits belong to Docker.

The stand covers one local database file in one running container. It does not
answer how to migrate a cluster safely or decide when a deployment may run a
migration.

## Why use yesh here?

The migrator changes the schema. Docker starts the process and mounts the
volume. But who checks that the right file is mounted, passes exact arguments,
and verifies the result? You usually write that part in Bash.

That wrapper is where the risk appears. A quoting mistake changes an argument.
An ordering mistake starts the migrator too early. A successful exit code does
not prove that the schema now has the intended change.

yesh makes the contract visible:

1. check the preconditions;
2. define the order and local lock;
3. show the external `argv` in `--plan`;
4. verify the result and return an error.

yesh does not manage schema versions or protect a cluster. The database,
migrator, and deployment system still own transactions, distributed locking,
backups, rollback, and release ordering.

## What happens

The image contains:

- a statically built `yesh` binary;
- the external `sqlite3` utility;
- a yesh script;
- a SQL migration file.

The database lives in a mounted Docker volume. On startup yesh:

1. checks that the database and migration exist;
2. obtains a local non-blocking lock;
3. checks whether the migration was already applied;
4. invokes external `sqlite3` with explicit arguments and redirection;
5. verifies the expected column;
6. returns an understandable exit code.

The responsibilities are separated:

| Participant | Responsibility |
|---|---|
| yesh | step ordering, local locking, branching, exact `argv`, result verification, `--plan`, and exit code |
| `sqlite3` and the application | database file, SQL, version tracking, and transaction |
| Docker | image, process, volume, and container limits |

yesh does not know SQLite's schema model and does not execute SQL itself. If the
external migrator later changes to PostgreSQL, yesh can keep the same role—check
the environment, invoke the migrator, and verify its exit code—but the stand,
readiness checks, and locking rules must change.

## Why there is no Bash

Instead of `ENTRYPOINT ["sh", "-c", "..."]`, the image starts yesh in exec form.
The scenario has no manual word splitting, hidden quoting, `trap`, `set -e`, or
implicit combination of exit codes. The external `sqlite3` call remains visible
in the plan.

The WSL2 build used `docker build --network host` only because the ordinary
build container could not resolve `proxy.golang.org` and the Alpine mirror.
That is a stand-specific networking issue, not a yesh requirement.

The stand can be reproduced with:

```sh
docker build --network host -f Dockerfile.sqlite-lab -t yesh-sqlite-lab .
docker volume create yesh-sqlite-lab-data
docker run --rm -i --read-only --network none --cap-drop=ALL \
  --entrypoint sqlite3 \
  -v yesh-sqlite-lab-data:/data \
  yesh-sqlite-lab /data/app.db < examples/container-sqlite-initial.sql
docker run --rm --read-only --network none --cap-drop=ALL \
  -e YESH_DB=/data/app.db \
  -e YESH_MIGRATION=/app/migration.sql \
  -e YESH_LOCK=/data/migration.lock \
  -v yesh-sqlite-lab-data:/data \
  yesh-sqlite-lab
```

For comparison, Bash would have to connect the checks, lock, exit codes, and
SQL calls by hand, approximately like this:

```sh
if test -f "$db" && test -f "$migration"; then
    if mkdir "$lock" 2>/dev/null; then
        trap 'rmdir "$lock"' EXIT
        if ! sqlite3 "$db" 'select ...' | grep -qx 1; then
            sqlite3 "$db" < "$migration" || exit $?
        fi
        sqlite3 "$db" 'select ...' | grep -qx 1 || exit 4
    else
        exit 5
    fi
fi
```

The comparison does not claim that Bash is invalid. It shows that in yesh the
ordering and argument boundaries are language rules, while `sqlite3` remains an
ordinary external command.

## Four proofs

- The positive run applies the migration and verifies the new column.
- The rerun sees the migration table and does not apply the migration again.
- The negative migration fails. `.bail on` prevents SQLite from continuing the
  SQL file after the error; the check confirmed that no partial column remained.
- `--plan` shows the lock, external `argv`, redirection, and both branches, but
  does not run `sqlite3` or change the volume.

The initial SQL schema is sent through stdin with `docker run -i`; without `-i`,
the preparation container may exit successfully while leaving the volume empty.
That belongs to stand setup, not to the yesh scenario.

## Proven boundary

The stand proves only a one-shot operation on Alpine 3.24 with a local SQLite
file in a named Docker volume on Linux/amd64. It does not prove a Kubernetes
`Job`, PostgreSQL, distributed locking, CSI/NFS, `fsGroup`, rootless storage, or
universal operation in an image without a shell.

`with lock` protects only competing local runs on the accessible path. Across
multiple nodes, the database, migrator, or external `Job`—not yesh's local
lock—must provide the single-run guarantee.

On Linux, yesh implements this lock with `flock` on an open file descriptor.
The kernel releases the lock when the process exits, including after `SIGKILL`.
The lock-path file may remain, but its existence does not mean that a process
still holds the lock. This stand does not yet test a killed process or an
abruptly terminated container; those cases need a separate interruption-and-
rerun test.

Stand sources: [Dockerfile.sqlite-lab](../../Dockerfile.sqlite-lab),
[yesh script](../../examples/container-sqlite-migrate.yesh),
[positive migration](../../examples/container-sqlite-migration.sql), and
[negative migration](../../examples/container-sqlite-failing-migration.sql).
