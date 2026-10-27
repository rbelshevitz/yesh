# Библиотеки yesh

## Решение для MVP

### Стандартная библиотека Go — обязательно

- `os/exec` — запуск процессов, отдельные аргументы, stdin/stdout/stderr;
- `context` — timeout и отмена команд;
- `io` — соединение pipe и потоков;
- `bufio` — bounded streaming reads for logs and large records;
- `encoding/json` — parsing `journalctl --output=json-seq` records;
- `os`, `path/filepath` — окружение, файлы и рабочий каталог;
- `flag` — `--plan`, `--verbose`, `--version`;
- `syscall`/`os.Process` — exit status и базовое управление процессами;
- `testing` — unit и integration tests.

`os/exec` намеренно не вызывает системный shell и не реализует pipeline,
globbing или redirection. Это именно то, что нужно yesh: эти возможности должен
контролировать собственный runtime.

### Парсер — сначала свой lexer/parser

У yesh необычная граница: после `if` и `for` идёт yesh-синтаксис, а строка
команды содержит shell-подобные pipes и redirections. Поэтому на первом этапе
лучше написать небольшой lexer и recursive-descent parser самим. Это сохранит
понятную диагностику и не заставит подгонять язык под чужую грамматику.

Кандидат для следующего этапа — `github.com/alecthomas/participle/v2`.
Participle строит AST из Go-структур и поддерживает собственный lexer, позиции
ошибок и stateful lexing. Подключать его стоит после стабилизации грамматики.

### Интерактивный режим

- `golang.org/x/term` — terminal detection, raw mode, размер терминала,
  ввод паролей и базовый readline;
- `github.com/chzyer/readline` — если понадобятся готовые history и completion;
- `github.com/creack/pty` — только позже, для полноценного запуска
  интерактивных программ и job control.

Файловый режим не должен зависеть от readline или PTY.

### POSIX process control — для production runtime

`os/exec` достаточно для обычного запуска, но полноценный shell позже потребует
`golang.org/x/sys` с OS-specific build tags для process groups, сигналов и
завершения всего pipeline по timeout. На Windows это будет отдельная ветка
реализации.

### Linux process/device diagnostics

- `github.com/prometheus/procfs` — кандидат для чтения `/proc` и `/sys`,
  процессов, mountinfo и block devices. API полезный, но сам проект предупреждает,
  что он пока может ломаться между версиями; до стабилизации yesh лучше держать
  тонкий собственный слой и использовать procfs как источник идей/тестов.
- `go.pdmccormick.com/initramfs` — кандидат для чтения и проверки Linux
  initramfs `newc`/`crc` CPIO. Это хороший вариант для `inspect`, но не backend
  dracut/mkinitcpio и не замена их сборщикам.
- `github.com/coreos/go-systemd` — кандидат для systemd D-Bus, journald и
  unit files. `sdjournal` требует CGO, поэтому для кроссплатформенного MVP
  предпочтительнее сначала запускать `journalctl` через безопасный argv или
  использовать D-Bus только в Linux-specific пакете.
- `github.com/godbus/dbus/v5` — общий native Go D-Bus transport; пригоден для
  systemd API, но не нужен для первой версии `systemctl`/`journalctl`
  упрощайзеров.
- libkmod — нативная C-библиотека для модулей, зависимостей и aliases. Go
  bindings существуют, но требуют CGO и платформенного libkmod; MVP использует
  `modprobe -n -v`, `--show-depends`, `modinfo`, `lsmod` как внешние команды с
  фиксированным argv. Прямой libkmod backend можно добавить позже для Linux.

### Initramfs backend decision

Не искать одну «универсальную» библиотеку для dracut и mkinitcpio. Они имеют
разную модель: dracut — модули, drivers и hostonly; mkinitcpio — hooks,
`MODULES`, `FILES` и `BINARIES`. yesh должен иметь общий typed intent и два
маленьких backend’а, которые строят plan и вызывают соответствующий tool.

Для проверки готового образа достаточно:

```text
dracut → lsinitrd → CPIO inspector
mkinitcpio → lsinitcpio → CPIO inspector
```

Собственный CPIO inspector нужен для read-only проверки содержимого и
checksum, но не для незаметного изменения boot image.

### Тестирование CLI

`github.com/rogpeppe/go-internal/testscript` хорошо подходит для сценариев:

```text
exec yesh deploy.yesh --plan
stdout 'systemctl restart api'
! stderr .
```

Он создаёт временную файловую систему, проверяет stdout/stderr и exit status.
Для unit-тестов AST и runtime достаточно стандартного `testing`.

## Что пока не подключать

### Cobra

Для первой версии у yesh несколько глобальных флагов, поэтому хватит `flag`.
Cobra имеет смысл, если появятся настоящие подкоманды вроде `yesh fmt`,
`yesh check`, `yesh run` и `yesh explain`.

### Tree-sitter

Tree-sitter полезен для incremental parsing, редакторных подсказок и IDE.
Но для MVP он преждевременен: добавляет отдельный grammar toolchain и не решает
сам по себе семантику Unix-команд. Вернуться к нему можно при создании LSP.

### Shell parser libraries

Парсеры Bash не подходят как основа: yesh не должен наследовать Bash-грамматику.
Библиотеки, которые только разбивают shell-команду на слова, также не заменяют
собственный AST для yesh.

## Итоговый стек

```text
Go stdlib
  ├─ parser/runtime/processes
  ├─ os/exec + context + io
  └─ testing

x/term                         → базовый REPL позже
chzyer/readline                → history/completion позже
go-internal/testscript         → сценарные тесты
participle/v2                  → кандидат после стабилизации grammar
tree-sitter                    → кандидат для LSP, не runtime
golang.org/x/sys               → process groups/signals, следующий runtime-слой
prometheus/procfs               → Linux /proc,/sys diagnostics, кандидат
go.pdmccormick.com/initramfs    → CPIO/initramfs inspect, кандидат
coreos/go-systemd / godbus      → systemd D-Bus, Linux-specific позже
```
