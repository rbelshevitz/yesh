# Linux integration: process lifecycle

Проверки нужны, чтобы timeout останавливал всю pipeline, раннее закрытие
reader не оставляло зависший writer, а повторные запуски не расходовали
дескрипторы.

Например, раннее закрытие reader:

```yesh
yes | head -n 1 > /dev/null
```

Сравнение с Bash:

```bash
set -o pipefail
yes | head -n 1 > /dev/null
```

Правило: ошибка любой стадии pipeline, включая SIGPIPE writer, означает
ошибку всей команды. Успех `head` не скрывает SIGPIPE от `yes`.

## Запуск проверок

```bash
export GOCACHE="$PWD/.gocache"
export GOMODCACHE="$PWD/.gomodcache"
go test -trimpath ./...
go test -race -trimpath ./cmd/yesh -run 'Test(POSIX|Linux|Pipeline)' -count=3 -timeout=240s
go test ./cmd/yesh -run '^$' -fuzz FuzzPipelinePreflight -fuzztime=10s -parallel=2
go test ./cmd/yesh -run '^$' -fuzz FuzzLiteralArgumentRoundTrip -fuzztime=10s -parallel=2
go test ./cmd/yesh -run '^$' -fuzz FuzzConditionQuotedData -fuzztime=10s -parallel=2
go vet ./...
go build -trimpath ./cmd/yesh
```

`runner_linux_test.go` проверяет общий PGID стадий, изоляцию от группы
интерпретатора, reap прямых детей, SIGPIPE, timeout процесса и pipeline
с игнорированием TERM, завершение потомка через группу и число открытых
дескрипторов после повторных успешных и частичных запусков. Тестовые
shell fixtures только создают процессы; runtime запускает их через
`exec`, без shell backend. Timeout в тестах сокращён через внутренний
`runWithin`; пользовательский default остаётся пять минут, grace — две секунды.

Root, устройства, systemd и запись в cgroup для этих тестов не нужны.
CI Linux и macOS повторяет проверки под детектором гонок.

## Расширенные проверки POSIX

Общий набор `runner_posix_test.go` и `cli_posix_test.go` работает на POSIX;
Linux-проверки отдельно используют `/proc` и доступные команды Linux.
Покрыты:

- двоичные потоки 2 MiB, конвейеры из 1/2/3/8/32 стадий, медленный читатель
  и запись после закрытия читателя с игнорированием PIPE/TERM;
- наследование ввода и отдельного потока ошибок, EOF, пути с пробелами,
  добавление и обнуление файлов;
- FIFO, устройства, каталоги, символьные и жёсткие ссылки,
  совпадение входного и выходного файла без потери содержимого;
- коды 0/1/7/126/127, ошибка каждой стадии, остановка зависшего соседа,
  отмена до запуска и во время работы, отличие сигнала и timeout от false;
- отсутствие неявного запуска оболочки при неверном формате executable,
  ошибка старта после 1/2/5 запущенных процессов и сбор всех прямых детей;
- TERM/INT/HUP интерфейсу командной строки, изоляция от соседней группы,
  отсутствие наследования посторонних дескрипторов, повторная отмена без
  накопления дескрипторов, горутин и несобранных детей;
- минимальное окружение cron, пустой PATH и защита от неявного поиска в cwd,
  конфликт блокировки flock, внешний timeout и отсутствие изменений crontab;
- раскрытые аргументы `--plan`, отсутствие запуска/открытия файлов в плане,
  предварительный отказ без исполнения первой стадии и обнуления вывода;
- опасные стадии после раскрытия переменных, буквальные пустые аргументы,
  сохранение строк и списков, кавычки, операторы и подстановки как данные;
- read-only запросы dpkg-query и apt-cache при наличии команд.

Три проверки со случайно порождёнными данными проверяют разбор конвейеров,
точную передачу буквального аргумента и слова внутри условий в кавычках. Реальных сетевых
загрузок, установки пакетов, управления daemon и записи в cgroup нет.

Новые правила описаны в [защите конвейеров](pipeline-safety.ru.md),
[запуске из cron](cron.ru.md), [пакетных операциях](packages.ru.md)
и собственной [справке yesh(1)](../man/ru/yesh.1).

Последний общий прогон после полировки условий/exit и добавления Р.П.П.:
`go test -race -trimpath -json ./... -timeout=180s` с временным
`-modfile=/tmp/yesh-posix.mod` — успешно, 40.789 с. Прошли 70 тестовых функций
и 261 конечный случай; проверка sticky-каталога с двумя UID пропущена
без повышения привилегий. Проверены chmod/chown текущего владельца,
umask 077 и установка/снятие sticky-bit в собственных временных каталогах.
Реальные сценарии rsync/pv/rsnapshot, backup с восстановлением, конфигурация,
отмена задания и пять случаев newc/gzip/zstd/lsinitramfs описаны в
[Р.П.П.](practical-examples.ru.md). Фиктивные `.ko` не исполняются и не загружаются в ядро.

После этого пользователь отдельно запустил `TestPOSIXStickyForeignEntries`
с повышенными правами вне sandbox агента и предоставил результат PASS
(0.01 с), включая обе проверки `remove` и `rename`. Таким образом,
ограничения для второго UID и успешное удаление после снятия sticky-bit
подтверждены отдельным прогоном. Это не повтор общего прогона под
детектором гонок; его показатели выше остаются прежними.

## Проверенное окружение, 2026-10-27

- Ubuntu 24.04.4 LTS, WSL2 kernel `6.18.54.1-microsoft-standard-WSL2`.
- `/proc`, `/proc/self/fd`, `/sys/module` доступны для чтения.
- cgroup v2 доступна; controllers: cpuset, cpu, io, memory, hugetlb, pids, rdma.
  Mount read-only: delegation и изменение лимитов здесь не проверяются.
- procps 4.0.4, kmod 31, udevadm 255, tmux 3.4 присутствуют;
  `screen` отсутствует. `findmnt`, `lsblk`, `journalctl` доступны.
  `rsyslogd` установлен; `udevadm info` читает sysfs для loopback,
  `modprobe -n -v loop` завершился успешно.
- В WSL включён systemd (`/etc/wsl.conf`, `systemd=true`). В sandbox агента
  PID 1 — `codex`, cgroup процесса — `/non-systemd`, поэтому именно из sandbox
  `systemctl is-system-running` получает `Failed to connect to bus:
  Operation not permitted`; это ограничение агента, а не отсутствие systemd в WSL.
  Чтение journal доступно.
- Каталог модулей текущего ядра присутствует. Загрузка/выгрузка модулей,
  изменение cgroup и запуск tmux sessions не выполнялись.
- Пользователь установил pv 1.8.5, rsnapshot 1.4.5, lsinitramfs/cpio/zstd;
  rsync 3.2.7 доступен. Практические проверки этих утилит прошли.
- Установлен Go 1.22.2, проект требует 1.27. Загрузка toolchain недоступна
  из sandbox. Локальные проверки используют временный modfile с `go 1.22`;
это не подтверждает сборку Go 1.27.

Локально прошли `go test -race -trimpath ./... -count=3 -timeout=120s`,
повторный race-прогон после добавления проверки ошибки открытия редиректа,
`go vet` и сборка с `-trimpath`. Во всех Go-командах использован временный
`-modfile=/tmp/yesh-linux-test.mod`; для сборки дополнительно понадобился
`-buildvcs=false`, поскольку Git-метаданные workspace недоступны.
Собранный бинарь выполнил успешную pipeline; пример SIGPIPE выше вернул
exit code 1 и остановил скрипт до следующей команды.

Эти проверки подтверждают Linux process groups и сигналы в данном WSL
sandbox. Systemd transient units, cgroup delegation/limits, TTY/PTY,
tmux/screen lifecycle и реальные устройства остаются отдельными проверками.
