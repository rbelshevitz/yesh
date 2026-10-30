# Conditions, Blocks, and Script Exit

Conditions should read the same way during execution and in a plan. Quotes
separate text from language words, and a failed check is not silently treated
as a false condition.

```yesh
config = /etc/os-release
if exists $config:
    cat < config
else:
    echo "configuration file is missing"
    exit 2
```

Bash comparison:

```bash
set -euo pipefail
config=/etc/os-release
if test -e "$config"; then
    cat < "$config"
else
    echo 'configuration file is missing'
    exit 2
fi
```

`exists` accepts one string path. A missing target is false; access denial,
symlink loops, and other errors stop the script. The check follows symlinks, so
a link to a missing target is false. A list, regex, empty path, or NUL in the
path is an error. Write `exists path`; the old `exists(path)` form is rejected
with a correction hint.

## Regular-expression matching

```yesh
service = api-42
pattern = regex '^api-[0-9]+$'
if $service matches $pattern:
    echo "matching service"
```

Bash comparison: `[[ "$service" =~ ^api-[0-9]+$ ]]`.

`matches` is a separate word between a text value and a regex. Words inside
quotes remain data, so `if printf '%s' 'a matches b':` checks `printf`'s status,
not a regex. The inline `regex '...'` form is also allowed and has the same
engine and limits.

An ordinary nonzero external command status means false. Termination by signal,
cancellation, timeout, and startup failure remain execution errors. In `--plan`,
external conditions are checked for valid structure but not run; both branches
are shown. `exists` and `matches` are evaluated without starting processes.

## Exit

```yesh
code = 7
exit $code
echo "this line is not executed"
```

Bash comparison: `code=7; exit "$code"`.

`exit $code` terminates the whole script, including nested `if` and `for` blocks,
with an explicit decimal code from `0` through `255`. It is a built-in control
operation, not an external process or an `if` result. In a plan it is displayed
without terminating the plan. A missing, negative, or out-of-range code, list,
pipeline, or redirect is an error.

## Block diagnostics

`if` and `for` require `:` and an indented body. `else:` is allowed only as the
branch immediately belonging to an `if` at the same level. Header errors are
found while parsing the complete file, before commands run. Words such as
`if`, `matches`, and `else` inside strings are data.

## Scoped locks

On Linux, `with lock path:` holds an exclusive advisory file lock for the
indented body and releases it on success, error, cancellation, or script exit:

```yesh
lock = "/var/lib/yesh/backup.lock"
with lock $lock:
    rsync -a --link-dest previous source/ temporary/
    publish temporary to current
```

The lock is non-blocking: a second process fails before entering the body.
`--plan` shows the lock path but does not acquire it. This is an orchestration
primitive, not a replacement for utility-specific locks or a permission
sandbox; the lock file's parent and ownership must be provisioned separately.

`publish source to destination` uses Linux/POSIX rename semantics. It requires
both paths to be on the same filesystem and does not silently fall back to a
copy. It provides atomic namespace visibility, not a power-loss durability
guarantee; fsync of files and directories remains an explicit future policy.

Missing files and denial while inspecting a parent directory are different
`stat` results; the runtime does not hide that distinction.

### Retention preview

```yesh
rsync retention plan in $root keep $keep
```

This is a read-only preview. It lists only real directories named
`snapshot.<number>` older than the newest `keep` entries; staging names and
symlinks are rejected. The report labels the newest entries `KEEP` and the
older entries `PRUNE`; the latter is only a report label. The count must be
between 1 and 10000. The operation never removes or renames anything, even
outside `--plan`.

### Retention quarantine

```yesh
with lock $lock:
    rsync retention quarantine in $root keep $keep to $quarantine
```

The quarantine directory must already exist as a real direct child of `root`.
The operation reads only completed `snapshot.<number>` directories inside the
explicit lock and moves entries older than `keep` with `rename`. It never
deletes quarantine contents and has no copy fallback across filesystems.
`--plan` reports candidates without changing the filesystem.
