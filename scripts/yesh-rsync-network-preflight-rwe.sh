#!/bin/sh
set -eu
umask 077

yesh=${1:-}
example=${2:-}
if test -z "$yesh" || test -z "$example"; then
    echo "usage: $0 /path/to/yesh /path/to/rsync-network-preflight.yesh" >&2
    exit 2
fi
test -x "$yesh"
test -r "$example"

base=$(mktemp -d /tmp/yesh-rsync-network-preflight-rwe.XXXXXX)
trap 'rm -rf -- "$base"' EXIT INT TERM HUP
bin=$base/bin
mkdir "$bin"

cat >"$bin/getent" <<'EOF'
#!/bin/sh
printf '%s\n' getent >>"$TRACE"
printf '%s\n' '192.0.2.10 STREAM example.test'
exit "${GETENT_STATUS:-0}"
EOF
cat >"$bin/ip" <<'EOF'
#!/bin/sh
printf '%s\n' ip >>"$TRACE"
printf '%s\n' '192.0.2.10 dev fixture0 src 192.0.2.2'
exit "${IP_STATUS:-0}"
EOF
cat >"$bin/ssh" <<'EOF'
#!/bin/sh
printf '%s\n' ssh >>"$TRACE"
exit "${SSH_STATUS:-0}"
EOF
cat >"$bin/rsync" <<'EOF'
#!/bin/sh
printf '%s\n' rsync >>"$TRACE"
exit "${RSYNC_STATUS:-0}"
EOF
chmod 0755 "$bin"/*

run_case() {
    name=$1
    shift
    trace=$base/$name.trace
    : >"$trace"
    if env PATH="$bin:$PATH" TRACE="$trace" \
        YESH_RSYNC_DNS_NAME=example.test \
        YESH_RSYNC_ROUTE_TARGET=192.0.2.10 \
        YESH_RSYNC_HOST=fixture@example.test \
        YESH_RSYNC_REMOTE_PATH=/srv/data \
        YESH_RSYNC_SOURCE=fixture@example.test:/srv/data/ \
        YESH_RSYNC_TARGET="$base/target" \
        "$@" "$yesh" "$example" >"$base/$name.out" 2>"$base/$name.err"; then
        return 0
    fi
    return 1
}

run_case positive env GETENT_STATUS=0 IP_STATUS=0 SSH_STATUS=0 RSYNC_STATUS=0
test "$(tr '\n' ' ' <"$base/positive.trace")" = "getent ip ssh rsync "

if run_case dns-failure env GETENT_STATUS=1 IP_STATUS=0 SSH_STATUS=0 RSYNC_STATUS=0; then
    echo "ERROR: DNS failure was accepted" >&2
    exit 1
fi
test "$(tr '\n' ' ' <"$base/dns-failure.trace")" = "getent "

if run_case route-failure env GETENT_STATUS=0 IP_STATUS=1 SSH_STATUS=0 RSYNC_STATUS=0; then
    echo "ERROR: route failure was accepted" >&2
    exit 1
fi
test "$(tr '\n' ' ' <"$base/route-failure.trace")" = "getent ip "

if run_case ssh-failure env GETENT_STATUS=0 IP_STATUS=0 SSH_STATUS=1 RSYNC_STATUS=0; then
    echo "ERROR: SSH failure was accepted" >&2
    exit 1
fi
test "$(tr '\n' ' ' <"$base/ssh-failure.trace")" = "getent ip ssh "

if run_case rsync-failure env GETENT_STATUS=0 IP_STATUS=0 SSH_STATUS=0 RSYNC_STATUS=23; then
    echo "ERROR: rsync failure was accepted" >&2
    exit 1
fi
test "$(tr '\n' ' ' <"$base/rsync-failure.trace")" = "getent ip ssh rsync "

if run_case timeout-failure env GETENT_STATUS=0 IP_STATUS=0 SSH_STATUS=0 RSYNC_STATUS=30; then
    echo "ERROR: rsync timeout was accepted" >&2
    exit 1
fi
test "$(tr '\n' ' ' <"$base/timeout-failure.trace")" = "getent ip ssh rsync "

echo "RWE rsync network preflight order, no fallback, and staged failure matrix passed"
