#!/bin/sh
set -eu

# Стенд для examples/deploy-static-systemd.yesh.
#
# Запускать на самом systemd-узле от root: nginx -t должен открыть действующий
# PID-файл и все файлы, перечисленные конфигурацией. Скрипт не перезапускает
# nginx и не меняет его конфигурацию. Он создаёт только один временный каталог
# внутри корня сайта и удаляет его после проверки, если KEEP_FIXTURE не равен 1.
#
# Требуются: yesh, nginx, curl, mkdir, printf, ln, readlink, rm, mktemp.
# Пример для Debian с обычным корнем nginx:
#   sudo YESH_BIN=/path/to/yesh \
#     YESH_SCRIPT=/path/to/yesh/examples/deploy-static-systemd.yesh \
#     ./scripts/yesh-deploy-static-systemd-fixture.sh
#
# Если nginx обслуживает другой корень, задайте YESH_WEB_ROOT. URL должен
# соответствовать этому корню; по умолчанию используется http://127.0.0.1.

if [ "$(id -u)" -ne 0 ]; then
    echo "run this fixture as root so nginx -t can read its runtime files" >&2
    exit 2
fi

yesh_bin=${YESH_BIN:-yesh}
yesh_script=${YESH_SCRIPT:-examples/deploy-static-systemd.yesh}
web_root=${YESH_WEB_ROOT:-/var/www/html}
health_base=${YESH_HEALTH_BASE_URL:-http://127.0.0.1}
keep_fixture=${KEEP_FIXTURE:-0}

if [ ! -x "$yesh_bin" ] && ! command -v "$yesh_bin" >/dev/null 2>&1; then
    echo "yesh executable not found: $yesh_bin" >&2
    exit 2
fi
if [ ! -f "$yesh_script" ]; then
    echo "yesh example not found: $yesh_script" >&2
    exit 2
fi
if [ ! -d "$web_root" ]; then
    echo "web root is missing: $web_root" >&2
    exit 2
fi

fixture=$(mktemp -d "$web_root/yesh-deploy-systemd.XXXXXX")
cleanup() {
    if [ "$keep_fixture" = 1 ]; then
        echo "fixture kept at $fixture"
    else
        rm -rf -- "$fixture"
    fi
}
trap cleanup EXIT HUP INT TERM
# mktemp intentionally creates 0700; nginx's worker must be able to traverse
# this disposable document-root while the fixture is being served.
chmod 0755 "$fixture"

mkdir -p "$fixture/incoming/release-1" "$fixture/releases/release-0"
printf '%s\n' 'old release' > "$fixture/releases/release-0/index.html"
printf '%s\n' 'old health' > "$fixture/releases/release-0/health.html"
printf '%s\n' 'new release' > "$fixture/incoming/release-1/index.html"
printf '%s\n' 'new health' > "$fixture/incoming/release-1/health.html"
ln -s "$fixture/releases/release-0" "$fixture/current"

export YESH_RELEASE="$fixture/incoming/release-1"
export YESH_RELEASE_INDEX="$fixture/incoming/release-1/index.html"
export YESH_RELEASE_FINAL="$fixture/releases/release-1"
export YESH_CURRENT="$fixture/current"
export YESH_CURRENT_NEXT="$fixture/current.next"
export YESH_PREVIOUS="$fixture/releases/release-0"
export YESH_DEPLOY_LOCK="$fixture/deploy.lock"
export YESH_HEALTH_URL="$health_base/$(basename "$fixture")/current/health.html"

echo '--- plan ---'
"$yesh_bin" --plan "$yesh_script"

echo '--- positive publication ---'
"$yesh_bin" "$yesh_script"
test "$(readlink "$YESH_CURRENT")" = "$YESH_RELEASE_FINAL"
test "$(curl --fail --silent --show-error "$health_base/$(basename "$fixture")/current/index.html")" = 'new release'
test ! -e "$YESH_CURRENT_NEXT"
echo 'positive publication passed'

mkdir -p "$fixture/incoming/release-bad"
printf '%s\n' 'bad release' > "$fixture/incoming/release-bad/index.html"
printf '%s\n' 'bad health' > "$fixture/incoming/release-bad/health.html"
export YESH_RELEASE="$fixture/incoming/release-bad"
export YESH_RELEASE_INDEX="$fixture/incoming/release-bad/index.html"
export YESH_RELEASE_FINAL="$fixture/releases/release-bad"
export YESH_PREVIOUS="$fixture/releases/release-1"
export YESH_HEALTH_URL="$health_base/$(basename "$fixture")/current/does-not-exist"

echo '--- failed HTTP and rollback ---'
if "$yesh_bin" "$yesh_script"; then
    echo 'expected HTTP check failure' >&2
    exit 1
fi
test "$(readlink "$YESH_CURRENT")" = "$fixture/releases/release-1"
test "$(curl --fail --silent --show-error "$health_base/$(basename "$fixture")/current/index.html")" = 'new release'
test ! -e "$YESH_CURRENT_NEXT"
echo 'HTTP rollback passed'

echo '--- interruption after current switch ---'
mkdir -p "$fixture/bin" "$fixture/incoming/release-interrupted"
printf '%s\n' 'interrupted release' > "$fixture/incoming/release-interrupted/index.html"
printf '%s\n' 'interrupted health' > "$fixture/incoming/release-interrupted/health.html"
printf '%s\n' '#!/bin/sh' 'printf "%s" "$$" > "$YESH_TEST_CURL_PID"' 'sleep 30' > "$fixture/bin/curl"
chmod 0755 "$fixture/bin/curl"

export YESH_RELEASE="$fixture/incoming/release-interrupted"
export YESH_RELEASE_INDEX="$fixture/incoming/release-interrupted/index.html"
export YESH_RELEASE_FINAL="$fixture/releases/release-interrupted"
export YESH_PREVIOUS="$fixture/releases/release-1"
export YESH_HEALTH_URL="$health_base/$(basename "$fixture")/current/health.html"
export YESH_TEST_CURL_PID="$fixture/curl.pid"

PATH="$fixture/bin:$PATH" "$yesh_bin" "$yesh_script" > "$fixture/interruption.log" 2>&1 &
deploy_pid=$!
switched=0
i=0
while [ "$i" -lt 100 ]; do
    if [ "$(readlink "$YESH_CURRENT" 2>/dev/null || true)" = "$YESH_RELEASE_FINAL" ]; then
        switched=1
        break
    fi
    if ! kill -0 "$deploy_pid" 2>/dev/null; then
        break
    fi
    i=$((i + 1))
    sleep 0.1
done
if [ "$switched" -ne 1 ]; then
    cat "$fixture/interruption.log" >&2
    kill "$deploy_pid" 2>/dev/null || true
    wait "$deploy_pid" 2>/dev/null || true
    echo 'did not observe current switch before interruption' >&2
    exit 1
fi

curl_pid=$(cat "$YESH_TEST_CURL_PID" 2>/dev/null || true)
kill "$deploy_pid" 2>/dev/null || true
if [ -n "$curl_pid" ]; then
    kill "$curl_pid" 2>/dev/null || true
fi
wait "$deploy_pid" 2>/dev/null || true
test "$(readlink "$YESH_CURRENT")" = "$YESH_RELEASE_FINAL"
test ! -e "$YESH_CURRENT_NEXT"
echo 'interruption boundary passed: current stays switched without automatic rollback'
