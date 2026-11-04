# yesh

![Go](https://img.shields.io/badge/Go-1.21-00ADD8.svg)
[![CI](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/yesh/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

yesh — читаемый язык автоматизации Linux и предварительных проверок.

> Bash без исторической синтаксической сложности.

yesh — язык ограниченных Linux-операций, в которых видно, что будет запущено, что
изменится и как будет проверен результат. Команды остаются обычными Linux-
командами, а сценарий явно показывает предусловия, порядок действий и границы
ошибки. yesh не заменяет Bash и Ansible. Он нужен для другого: описывать и
выполнять небольшие Linux-операции, где ошибка может дорого обойтись: до
запуска виден план, а после запуска — проверяемый результат. В таких сценариях
yesh проще читать и проверять, чем эквивалентный Bash-сценарий.

## Для чего он нужен

yesh рассчитан на небольшие административные сценарии: резервное копирование,
публикацию конфигурации, проверку служб, диагностику и отдельные шаги CI/CD.
Это не интерактивная оболочка и не система управления парком машин.

Главная ниша — сценарий примерно на 100–200 строк, который уже трудно читать
в Bash, но для которого Ansible или отдельное приложение избыточны.

    if grep -q '^mode=production$' config.next:
        install -m 0640 -- config.next /etc/myapp.conf.next
        publish /etc/myapp.conf.next to /etc/myapp.conf
        service restart myapp.service until active within 10s

Внешние команды видны, аргументы не собираются в строку для повторного разбора,
а `--plan` показывает порядок действий до запуска.

Ценность yesh проверяется не длиной файла и не количеством встроенных команд.
Для одного и того же ограниченного сценария важно, можно ли заранее увидеть
передаваемые аргументы, понять сценарий отказа, проверить сохранность предыдущего
состояния и воспроизвести отчёт по сохранённым наблюдениям. Это критерий
отдельных ответственных сценариев, а не утверждение, что yesh лучше Bash во
всех задачах.

См. [аккуратную базовую реализацию на Bash](examples/bash-rsync-incremental-baseline.sh)
рядом с соответствующим [сценарием yesh](examples/rsync-incremental.yesh). Это
не карикатура на Bash: пример показывает ручную служебную логику вокруг
`rsync`, чтобы разницу в читаемости и проверяемости можно было оценить на одной
операции.

## Быстрый старт

Требуется Go 1.21 или новее; для функций, зависящих от Linux, нужен Linux:

    go build -trimpath -o yesh ./cmd/yesh
    ./yesh examples/hello.yesh
    ./yesh --plan examples/hello.yesh

Начните с [быстрого старта](docs/quickstart.ru.md), затем откройте
[учебник](docs/syntax-guide.ru.md) и [практические примеры](docs/practical-examples.md).

Все файлы из `examples/` проверяются на разбор. Часть примеров требует
переменных окружения `YESH_*`, внешних утилит, работающей службы, SSH или
отдельного стенда; без этих условий они останавливаются до действия. Это
ограничение окружения, а не ошибка синтаксиса.

## Ресурсно ограниченные Linux-системы

yesh поддерживает проверки служб только для чтения под systemd и OpenRC:
показывает платформу, супервизор, PID, cgroup, процессы и конечные измерения
ресурсов. Недоступные данные помечаются как `unavailable`; это не постоянный
мониторинг. См. [пример OpenRC](examples/openrc-auto-resource-check.yesh) и
[техническую заметку](docs/underthehood/resource-constrained-os.ru.md).

Команды остаются внешними: yesh не переписывает `rsync`, `mount`, `sed`, `awk`,
менеджер пакетов или оболочку. Наблюдение подтверждает только среду, где оно
собрано; пропуск зависимости не считается доказательством работы.

## Документация

- [быстрый старт](docs/quickstart.ru.md);
- [учебник языка](docs/syntax-guide.ru.md);
- [практические примеры](docs/practical-examples.md);
- [systemd и измерение ресурсов](docs/systemd.ru.md);
- [OpenRC и ресурсно ограниченные системы](docs/underthehood/resource-constrained-os.ru.md);
- [корпус сравнения с Bash](docs/underthehood/bash-corpus.ru.md);
- [границы безопасности](SAFETY.ru.md);
- [дизайн языка](DESIGN.ru.md);
- [текущий план](docs/TODO.ru.md);
- [страница справки](man/ru/yesh.1).

Подробные результаты Linux-проверок находятся в
[техническом разделе](docs/underthehood/README.ru.md). Проект находится в
активной разработке; текущая заявленная платформа — Linux/amd64.

[English README](README.md) · [README на иврите](README.he.md)

## Проверка из рабочей копии

    GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go test ./...
    GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go vet ./...
    GOCACHE=$PWD/.gocache GOMODCACHE=$PWD/.gomodcache go build -trimpath ./cmd/yesh

Лицензия: MIT.
