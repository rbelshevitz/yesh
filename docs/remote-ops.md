# Remote Operations: SSH, SCP, rsync, and mount

Remote administration is a primary practical use case for yesh.

```yesh
hosts = [web-1, web-2]

for host in $hosts:
    ssh host systemctl restart api
    scp build/api host:/srv/api/api
    rsync -az build/ host:/srv/api/
```

`ssh`, `scp`, `rsync`, and `mount` remain Unix commands. yesh does not replace
their protocols or store keys: the OpenSSH agent, `~/.ssh/config`,
`known_hosts`, ProxyJump, and host-key policy remain external configuration.

## Typed remote paths

The target form is a checked value rather than string concatenation:

```yesh
target = remote host /srv/api
rsync -az build/ target
```

The runtime serializes it as `web-1:/srv/api` or
`[2001:db8::10]:/srv/api`. This is not a general object model.

## SSH

For `ssh host systemctl restart api`, the runtime builds the remote command
from separate arguments with POSIX-safe quoting. Shell operators, command
substitution, and nested shells are rejected.

These forms are rejected by default:

```yesh
ssh host bash -c text
ssh host "$(make_command)"
```

Interactive commands need a separate explicit mode such as `ssh -t`; ordinary
scripts must not hang waiting for remote input.

## SCP, rsync, and mount

`scp` must validate and canonicalize native `host:/path` values before use;
host-key checking must not be disabled. `rsync` receives separate arguments:

```yesh
rsync -a --delete build/ target
```

`--delete` is dangerous and must be visible in `--plan`. The distinction
between `build/` and `build` must survive path normalization. Authentication,
host verification, and secrets remain OpenSSH responsibilities.

Mount changes machine state and remains an explicit external command in the
MVP:

```yesh
mount -t nfs server:/exports/app /mnt/app
sshfs host:/srv/data /mnt/data
umount /mnt/data
```

## First remote MVP limits

- hosts are finite lists and execution is sequential;
- explicit parallelism comes later with a concurrency limit;
- credentials come only from OpenSSH agent/config and the environment;
- no `sshpass` and no `StrictHostKeyChecking=no`;
- every remote command has a timeout and complete exit status;
- one host failure stops rollout unless `collect-errors` is explicit.
