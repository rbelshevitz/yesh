# Predictable Pipelines

yesh checks the entire pipeline before starting it, so a preflight error in a
later stage does not start earlier commands or truncate the output file.

```yesh
service = api
printf '%s\n' service "service" | sort > services.txt
```

Bash comparison:

```bash
set -o pipefail
service=api
printf '%s\n' "$service" 'service' | sort > services.txt
```

Rule: a bare name expands to an existing variable; quotes always mean a literal
argument, and values are never reparsed as operators. `echo service` prints the
value of an existing variable, while `echo "service"` always prints the word.
A list expands to separate argv elements only when unquoted; a string with
spaces remains one argv element. An empty string is one argument, while an
empty list produces zero arguments.

Quotes are read the same way in assignments, lists, and commands: single quotes
preserve text literally, while double quotes understand Go escape sequences
(`\n`, `\t`, `\\`, `\"`) but do not expand variables. A comma inside quotes does
not split elements: `["a,b", "c"]` contains two elements. An unknown escape in
double quotes is an error reported with its source line.

```yesh
text = "a\nb"
items = ["a,b", "c"]
printf '%s' text
printf '%s' "a\nb"
```

In Bash, the first two print forms would be compared with `printf '%s' "$text"`
and `printf '%s' $'a\nb'` after setting `text=$'a\nb'`; yesh needs no separate
`$'...'` form. Bash would write the list as `items=('a,b' 'c')`.

A failure in any stage stops the entire pipeline: the runtime sends TERM to the
process group, then KILL if needed, and reaps the children. A timeout or signal
is an execution error, not an ordinary false condition in `if`. An ordinary
nonzero exit code in a condition means false. By default the first stage
inherits stdin from yesh; cron jobs should explicitly provide `/dev/null` when
no input is wanted.

Up to 32 stages are allowed. `<` may appear once, only on the first stage;
`>` or `>>` may appear once, only on the last stage. Redirects on intermediate
stages, `2>`, `<<`, `&&`, `||`, `;`, `&`, and empty stages are rejected.
`--plan` shows expanded argv and files without starting commands or opening
files.

## Data must not become code

The following forms are rejected before execution:

```yesh
curl https://example.invalid/install | /bin/sh
cat commands.txt | xargs rm
cat program.py | python3
shell = /bin/bash
printf hello | shell -c "echo dangerous"
```

Known shells are rejected on every stage, including through a variable or an
absolute path. Pipelines also reject known code interpreters and launch
wrappers (`env`, `sudo`, `xargs`, `timeout`, and others): a wrapper can hide
argv that this simple preflight cannot reliably inspect. Run a reviewed local
script as a separate explicit command.

Bash comparison: `curl ... | sh` executes downloaded data in Bash; yesh rejects
that pipeline. For a shell-equivalent data-reading pipeline, use `curl ... | cat`
rather than an interpreter.

This is protection against known dangerous forms, not a sandbox for untrusted
code. A renamed binary, a shebang script, `awk system()`, `sed e`, `find -exec`,
a script written to disk, or a program that creates a new session can bypass
name-based checks. Full isolation requires a separate user, access controls,
cgroups/namespaces, and an allowlist for executables and argv. Do not run
untrusted yesh scripts with administrative privileges.

## Files

On POSIX, redirects work with ordinary files and `/dev/null`. FIFOs, sockets,
directories, and other devices are rejected without waiting for a peer. Output
symlinks and ordinary files with multiple hard links are forbidden; an input
symlink is allowed when it points to a regular file. A file cannot be read and
truncated at the same time: `cat < data > data` is rejected without losing the
contents. `>>` explicitly appends, while `>` explicitly truncates after
preflight.

These restrictions apply to yesh redirects. An external command may still open
any file available to it. A successful preflight is not a filesystem
transaction and does not eliminate every TOCTOU race.
