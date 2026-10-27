# Текстовые фильтры, процессы и cgroups

## Решение по `sed` и `awk`

`sed` и `awk` остаются доступными Unix-командами. Их полное встраивание
противоречит правилу «один способ»: пришлось бы поддерживать две отдельные
грамматики, quoting и переносы состояния. GNU `sed` — полноценный stream
editor, а `awk` — язык правил «pattern → action», а не простой флаг команды.

В yesh нужны три понятных операции, а не копия этих языков:

```yesh
search "error" in /var/log --type log
replace "old-host" with "new-host" in config/*.conf
fields 1,3 from /var/log/app.tsv separated by tab
```

Правила:

- `search` только читает и возвращает совпадения;
- `replace` сначала строит план и по умолчанию не меняет файлы;
- применение требует `apply` и атомарной записи, с сохранением mode/owner;
- glob раскрывается runtime, не shell; пустой glob — ошибка;
- бинарные файлы, symlink и файлы вне корня операции требуют явного решения;
- сложная логика остаётся в отдельном `.awk`/`.sed` файле и запускается как
  обычная команда, а не прячется в yesh-строке.

`rg` полезен как реализация read-only поиска: он умеет ограничивать типы и
глубину, пропускает бинарные файлы и по умолчанию не изменяет файлы. Его
`--replace` меняет только вывод, поэтому yesh не должен называть это записью.
[GNU sed](https://www.gnu.org/software/sed/manual/sed.html),
[GNU awk](https://www.gnu.org/software/gawk/manual/gawk.html),
[ripgrep guide](https://ripgrep.dev/docs/guide/)

## Объект поиска процесса

Не делать прямой аналог `pkill pattern`: короткое имя процесса ограничено
15 символами, а PID может быть повторно выдан другому процессу. `pgrep` полезен
как источник критериев, но yesh обязан повторно проверить выбранный процесс
перед сигналом. [pgrep/pkill](https://man7.org/linux/man-pages/man1/pgrep.1.html)

Целевой синтаксис:

```yesh
api = processes where name is "api" and user is "app"
show api
signal TERM to api
wait api stopped within 30s
```

Безопасная модель:

- `processes where ...` возвращает набор записей, а не строку stdout;
- поля: `pid`, `ppid`, `name`, `exe`, `argv`, `uid`, `user`, `state`,
  `start_time`, `pgid`, `sid`, `cgroup`;
- критерий по умолчанию точный; regex и полная command line требуют явного
  слова `matches`;
- `signal` сначала показывает количество и identity процессов;
- сигнал пустому набору — ошибка, сигнал нескольким — ошибка без `many`;
- запрещены PID 1, сам yesh и его предки без отдельного unsafe-режима;
- после поиска проверяются `start_time` и executable, чтобы PID reuse не привёл
  к сигналу чужого процесса;
- Linux MVP использует pidfd, если доступен; обычный `kill(pid)` — fallback с
  повторной проверкой. `pidfd_open` возвращает file descriptor, который
  однозначно ссылается на процесс и сам тоже должен быть закрыт.

Для групп процессов предпочтительнее `process group`/cgroup, а не перебор PID:

```yesh
signal TERM to group api
wait group api empty within 30s
```

## cgroups и systemd

cgroup v2 — иерархия, где каждый процесс состоит ровно в одной группе;
`cgroup.procs` содержит PID-ы группы, а контроллеры ограничивают ресурсы
иерархически. Нельзя бездумно писать в `/sys/fs/cgroup`: нужны права,
делегация и понимание владельца дерева. [kernel cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)

Для обычной админки yesh предпочитает systemd как владеющий слой:

```yesh
job = scope "yesh-backup"
run job rsync -a /srv/data /mnt/backup
limit job memory 512MiB
limit job cpu 50%
wait job finished within 1h
```

Реализация сначала может раскладывать это в `systemd-run` transient scope/service
с явными `--wait`, `--pipe` и resource options. `systemd-run` запускает команду
в transient unit, а systemd resource control применяет ограничения к unit и
его дочерним процессам. [systemd-run](https://www.freedesktop.org/software/systemd/man/258/systemd-run.html),
[systemd resource control](https://www.freedesktop.org/software/systemd/man/258/systemd.resource-control.html)

Диагностика:

```yesh
show cgroup job
top cgroups by memory once
tree cgroup job
```

Это соответствует `systemd-cgtop` и `systemd-cgls`, но yesh должен всегда
ограничивать итерации: бесконечное наблюдение и `follow` не входят в MVP.

## Нельзя в MVP

- `pkill` по неограниченному regex;
- массовый сигнал без отображения выбранных процессов;
- ручное редактирование cgroup-файлов без явного unsafe-режима;
- `kill -9` как действие по умолчанию;
- бесконечные `watch`, `journalctl -f`, `top` и `systemd-cgtop`;
- превращать stdout `ps`/`awk` в доверенные объекты без схемы и проверки.

## Память процесса и фоновые терминальные сессии

### Метрики памяти

Типизированный процесс должен различать:

```text
rss       resident memory, реально находящаяся в RAM
vsz       virtual address space
pss       proportional share с учётом shared pages
swap      swapped pages, если доступно
memory    cgroup memory.current для группы
```

`RSS` нельзя называть полной стоимостью процесса: shared pages, page tables,
kernel stack и служебная память могут учитываться отдельно. `VSZ` не означает
физическое потребление. `PSS` полезнее для сравнения сервисов, но требует
`/proc/<pid>/smaps_rollup` или аналогичного источника. [ps memory fields](https://man7.org/linux/man-pages/man1/ps.1.html)

Целевой запрос:

```yesh
workers = processes where name matches 'worker'
show workers fields pid, rss, pss, cpu, state, cgroup
sort workers by rss descending
```

Для одного snapshot достаточно `/proc`/`sysfs`; для динамики нужны два снимка
с интервалом и явная delta. Бесконечный `top` внутри скрипта запрещён.

### tmux и screen

Это не обычные фоновые процессы, а владельцы PTY и виртуальных терминалов.
Для долгих unattended jobs предпочтительнее systemd scope/service: там есть
timeout, cgroup, exit status и logs. tmux/screen нужны для интерактивных
rescue/deploy-сессий.

Целевой API:

```yesh
session = tmux session "deploy-api"
session.start command ssh host "tail -f /var/log/app.log"
session.capture last 100 lines
session.send "status" enter
session.stop within 10s
```

Правила:

- сначала `list`/`show` и проверка точного session/window/pane ID;
- `capture` — read-only и ограничен строками/байтами;
- `send` запрещён в safe mode для произвольного текста: input может выполнить
  команду или terminal escape sequence;
- команды запускаются как argv при создании session, не через `send-keys`;
- owner, tty, PID/PGID и cgroup видны в результате;
- stop отправляет TERM группе и имеет timeout/escalation;
- секреты нельзя вводить через `send`/`stuff`;
- attach/follow не входит в MVP.

tmux предпочтительнее для backend: он предоставляет стабильные IDs `$session`,
`@window`, `%pane`, фильтры и `-F` machine-readable formats. `capture-pane -p`
подходит для ограниченного чтения, а `send-keys` — только для будущего
явного unsafe-режима. [tmux](https://man7.org/linux/man-pages/man1/tmux.1.html)

Для screen поддерживаем compatibility backend: `screen -ls`, точное имя через
`-S`, `-X hardcopy` для чтения и ограниченные команды управления. `stuff` не
используем как обычный способ запуска команд: это передача символов в PTY, а
не безопасный argv API. [screen](https://man7.org/linux/man-pages/man1/screen.1.html)
