#!/bin/sh
# B9 base RWE: archive read-only network observations without changing state.
# Usage: sh scripts/yesh-network-observation-rwe.sh ./yesh ./examples

set -eu
umask 077

yesh=${1:?usage: $0 /path/to/yesh /path/to/examples}
examples=${2:?usage: $0 /path/to/yesh /path/to/examples}
archive=${YESH_NETWORK_ARCHIVE:-./yesh-network-observation.tar}
test -x "$yesh"
test -r "$examples/network-interface-report.yesh"
test -r "$examples/network-route-report.yesh"
test -r "$examples/network-neighbour-report.yesh"
test -r "$examples/network-listener-report.yesh"
command -v ip >/dev/null
command -v ss >/dev/null
command -v head >/dev/null
command -v tar >/dev/null
command -v cp >/dev/null
test ! -e "$archive"

base=$(mktemp -d /tmp/yesh-network-observation-rwe.XXXXXX)
trap 'rm -rf -- "$base"' EXIT INT TERM HUP

run_plan() {
    name=$1
    example=$2
    "$yesh" --plan "$example" >"$base/plan-$name.out" 2>"$base/plan-$name.err"
    test ! -s "$base/plan-$name.err"
    grep -F 'PLAN' "$base/plan-$name.out" >/dev/null
}

run_report() {
    name=$1
    example=$2
    "$yesh" "$example" >"$base/report-$name.out" 2>"$base/report-$name.err"
    test ! -s "$base/report-$name.err"
}

run_plan interfaces "$examples/network-interface-report.yesh"
run_plan routes "$examples/network-route-report.yesh"
run_plan neighbours "$examples/network-neighbour-report.yesh"
run_plan listeners "$examples/network-listener-report.yesh"

run_report interfaces "$examples/network-interface-report.yesh"
run_report routes "$examples/network-route-report.yesh"
run_report neighbours "$examples/network-neighbour-report.yesh"
run_report listeners "$examples/network-listener-report.yesh"

test ! -e "$base/network.tar"

fake_bin=$base/fake-bin
mkdir -m 700 "$fake_bin"
printf '%s\n' '#!/bin/sh' 'exit 77' >"$fake_bin/ip"
chmod 700 "$fake_bin/ip"
set +e
PATH="$fake_bin:$PATH" "$yesh" "$examples/network-interface-report.yesh" >"$base/negative-ip.out" 2>"$base/negative-ip.err"
negative_status=$?
set -e
[ "$negative_status" -ne 0 ]
test ! -s "$base/negative-ip.out"

tar -cf "$base/network.tar" -C "$base" \
    plan-interfaces.out plan-interfaces.err \
    plan-routes.out plan-routes.err \
    plan-neighbours.out plan-neighbours.err \
    plan-listeners.out plan-listeners.err \
    report-interfaces.out report-interfaces.err \
    report-routes.out report-routes.err \
    report-neighbours.out report-neighbours.err \
    report-listeners.out report-listeners.err \
    negative-ip.out negative-ip.err
tar -tf "$base/network.tar" | grep -Fx 'report-interfaces.out' >/dev/null
tar -tf "$base/network.tar" | grep -Fx 'negative-ip.err' >/dev/null
test -s "$base/network.tar"
cp -- "$base/network.tar" "$archive"

echo 'RWE network observation plan, address/route/neighbour/listener reports, archive, and missing-ip negative passed'
echo 'interface addresses:'
cat "$base/report-interfaces.out"
echo "archive: $archive"
