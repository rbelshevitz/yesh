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

Consider a one-shot run. You mount a volume with the database and provide a
migration file. The wrapper must check the paths, wait for the database, prevent
two local runs, invoke the migrator, and verify the expected change. `sqlite3`,
or another external tool, already knows how to apply the migration. The problem
is the sequence around that tool.

Without a separate language, you usually write this sequence as a short Bash
script. It is easy to lose a quote, an argument boundary, an exit code, or an
ordering rule. The problem is sharper in a minimal image: familiar shell and
diagnostic tools may be absent, so you find the mistake only after the
container starts.

yesh makes the sequence explicit. The script shows which database it checks,
which `argv` it passes to `sqlite3`, when it takes the local lock, and which
result it verifies after the migration. `--plan` lets you inspect those actions
before the run.

That is the useful boundary for yesh: one understandable operational run on one
node or in one container. yesh does not choose schema versions or protect a
cluster. The database, migrator, and deployment system still provide distributed
locking, transactions, backups, schema rollback, and deployment ordering.

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

### What yesh does

In this example yesh owns the operational glue:

- refuse to start when the database or migration file is missing;
- prevent two local copies of the scenario from running at once;
- pass `sqlite3` separate arguments without another shell parse;
- stop when the external command fails;
- verify an expected result after a successful call;
- show the intended actions in `--plan`.

yesh does not own migration version selection, old/new schema compatibility,
arbitrary database transaction semantics, rollback of irreversible actions, or
the guarantee that only one copy runs across multiple nodes. Those decisions
belong to the database, external migrator, and deployment system.

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

Stand sources: [Dockerfile.sqlite-lab](../../Dockerfile.sqlite-lab),
[yesh script](../../examples/container-sqlite-migrate.yesh),
[positive migration](../../examples/container-sqlite-migration.sql), and
[negative migration](../../examples/container-sqlite-failing-migration.sql).
