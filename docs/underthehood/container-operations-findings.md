# Containerized operational pain map

Source review date: November 4, 2026.

This note collects real discussions and primary documentation for yesh's
longer-term research. It does not describe an implemented interface and does
not prove that yesh already runs inside containers.

## Main conclusion

The pain is not simply the absence of `bash`. It appears when a minimal
environment needs a one-shot operational sequence, while ordering, partial
failure, and result verification are hidden in shell glue.

The strongest direction is a separate one-shot operational image:

```text
yesh + explicitly required external tools + volume/network
→ preflight → action → result check → exit code
```

yesh must not restore a shell, replace missing `tar`/`cp`, become a container
runtime, or manage a cluster.

The testable hypothesis for yesh is narrower: it reduces the complexity of the
one-shot wrapper around an external command, not the complexity of the database
or container orchestrator. That wrapper checks preconditions, fixes ordering,
contains partial failure, exposes the invocation, and verifies the result. If
the migrator or deployment system already expresses these actions clearly,
yesh is not needed here.

## 1. Database migration as a one-shot operation

Two responsibilities must be separated. The migrator changes the schema and
knows which versions are applied. The operational scenario around it checks the
environment, waits for database availability, runs the migrator for the chosen
run, and passes on only a successful result.

Running migrations from an `initContainer` means running them for every Pod,
including restarts and scale-out. A separate `Job` removes part of the race but
adds creation order, completion waiting, reruns, and deployment integration.
That is a deployment-system concern, not something one local yesh lock solves.

- [Init Containers, Kubernetes](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/)
- [Jobs, Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [Migration through a Job or initContainer](https://www.reddit.com/r/kubernetes/comments/1v4d1m4/how_should_i_handle_database_migrations_in_a/)
- [Migration races and ordering across replicas](https://www.reddit.com/r/kubernetes/comments/1qdcpf2/how_do_you_guys_run_database_migrations/)
- [One-off migration task, Stack Overflow](https://stackoverflow.com/questions/37058812/how-best-to-run-one-off-migration-tasks-in-a-kubernetes-cluster)

yesh boundary: the external migrator and database own schema semantics, version
tracking, and transactions. The deployment system or migrator decides which
node runs the migration. A local `with lock` protects only processes that see
the same local path; it does not prove a single run across multiple nodes.

## 2. Volume permissions and non-root processes

`runAsUser`, `runAsGroup`, `fsGroup`, `subPath`, CSI, NFS, `hostPath`, and
rootless modes can produce different results. Recursive `chown` can be slow,
forbidden, or applied to the wrong path.

- [Security context and fsGroup, Kubernetes](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)
- [subPath and unexpected root ownership](https://www.reddit.com/r/kubernetes/comments/1j1lq1e/unexpected-subpath-behavior-in-kubernetes-auto-created-directories-with-root-ownership-and-permission-issues/)
- [Init-container permissions for chown, Stack Overflow](https://stackoverflow.com/questions/70717175/kubernetes-use-the-same-volumemount-in-initcontainer-and-container)
- [fsGroup does not always solve Filestore permissions](https://www.reddit.com/r/kubernetes/comments/1q6921a/best-practices-for-runasgroup-fsgroup-to-avoid-permissiondenied-on-filestore-mounts/)
- [Volume ownership-change policy, Kubernetes](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#configure-volume-permission-and-ownership-change-policy-for-pods)

yesh boundary: check the expected target, UID/GID, owner, and permissions; do
not promise a universal permission repair or silently perform a dangerous
`chown`.

## 3. Images without a shell

A distroless image may have no shell, package manager, or ordinary utilities.
Diagnostics then move to a separate debug image or ephemeral container.

- [Distroless contents and debug images](https://github.com/GoogleContainerTools/distroless)
- [Debugging a Pod without utilities, Kubernetes](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/)
- [Practical distroless debugging](https://www.reddit.com/r/docker/comments/1uz926r/if_i_go_distroless_and_cant_exec_in_how_do_you/)
- [Exec limitations in minimized containers](https://www.reddit.com/r/kubernetes/comments/15tpohb/tips_for_troubleshooting_minimized_containers/)

yesh boundary: investigate a finite report from a separate operational or debug
image. Do not make yesh a universal shell inside the application image.

## 4. PID 1 and signals

Shell-form `ENTRYPOINT` can leave the application as a shell child, making
signals and container shutdown surprising.

- [Dockerfile shell and exec forms](https://docs.docker.com/reference/dockerfile/)
- [JSON arguments and PID 1, Docker](https://docs.docker.com/reference/build-checks/json-args-recommended/)
- [PID 1, `--init`, and signals](https://docs.docker.com/reference/cli/docker/container/run)
- [Practical `exec` and SIGTERM discussion](https://www.reddit.com/r/docker/comments/1vdt7id/why_docker_stop_takes_ten_seconds_on_a_container/)

This is now a focused acceptance question for the one-shot container scenario.
The useful guarantee is narrow: if yesh is PID 1, it must cancel and clean up
the process group it started, wait for its children, and report interruption
without publishing success. This does not make yesh `tini`, a supervisor, or a
process manager.

## 5. Secrets, configuration, and certificates

A Secret mounted as a file may update, but the application must reread it. An
environment variable usually requires a restart. This leads to reload, `SIGHUP`,
Pod restart, and separate reloader tools.

- [Secrets as files or environment variables, Kubernetes](https://kubernetes.io/docs/concepts/configuration/secret/)
- [Updating a Secret does not automatically update application state](https://stackoverflow.com/questions/62913187/if-i-update-a-secret-does-it-automatically-gets-updated-to-all-resources-in-which-it-is-volume-mounted)
- [fsnotify, sidecars, and TLS reload](https://www.reddit.com/r/kubernetes/comments/1l1n0w/whats-the-best-approach-for-reloading-tls-certs-in-kubernetes-prod-fsnotify-on-parent-dir-vs-sidecar-based-reloads/)
- [Restarting after Secret or ConfigMap changes](https://www.reddit.com/r/kubernetes/comments/1hco0ge/reloadrestart-pod-on-file-changes/)

yesh boundary: a finite sequence of “check → call external reload or restart →
check state”. A watcher, reloader, and controller remain external systems.

## 6. Stack updates and host cleanup

On a single container host, a short cron script often pulls new images,
recreates changed containers, checks the result, and prunes unused objects. The
pain is the ordering and safety boundary: an old container may keep running,
`prune` may reclaim a surprising amount of space, and rollback may not be ready.

- [Pruning unused objects, Docker](https://docs.docker.com/engine/manage-resources/pruning)
- [Updating a Compose stack with a script](https://www.reddit.com/r/docker/comments/1k7gkny/using_docker_pull_but_with_a_compose_stack/)
- [Updater reported success while old containers kept running](https://www.reddit.com/r/selfhosted/comments/1vvr1u8/i_wrote_an_updater_that_printed_success_and_left/)
- [Unexpected Docker cleanup volume](https://www.reddit.com/r/selfhosted/comments/1fv7lih/reminder_to_prune_your_docker_hosts_once_in_a/)
- [Stop containers, back up, update, and clean in one script](https://www.reddit.com/r/selfhosted/comments/1fv7lih/reminder_to_prune_your_docker_hosts_once_in_a/)

yesh boundary: external `docker` or `podman` owns images and containers. yesh
may show an update plan, check space and state, wait for readiness, and defer
cleanup until success; it must not parse Compose as its own language or update
the entire host automatically.

## Research priority

1. An Alpine image without `bash`, a static yesh binary, external `sqlite3`, a
   temporary volume, and one migration;
2. the same ordering with PostgreSQL in a one-shot `Job`;
3. volume checks for a non-root process;
4. one external Compose-stack update with verification and deferred cleanup;
5. finite diagnosis of a shell-less image.

Each scenario needs a positive case, partial failure, rerun, a negative case
with no data change, and `--plan`. A skip caused by missing Docker, Podman,
Kubernetes, or an external tool is not evidence of working container behaviour.
