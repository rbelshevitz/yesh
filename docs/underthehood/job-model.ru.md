# Job model: процессы, pipeline, systemd и PTY

## Идея

Внутри runtime нужен не набор разрозненных «запусков команд», а единый
жизненный цикл `job`. Пользователь при этом не обязан видеть объектную модель:
`job` — внутренний runtime record, который связывает ресурсы и cleanup.

Один job может быть:

- одним внешним процессом;
- pipeline;
- systemd transient scope/service;
- tmux/screen session с PTY.

Во всех случаях yesh должен знать, как job началась, как её остановить, что
ждать, где её stdout/stderr, какой timeout, какие PID/PGID/cgroup и что
освободить при ошибке.

## Жизненный цикл

```text
planned
  → prepared
  → started
  → running
  → stop-requested
  → exited
  → reaped
  → cleaned
```

Переход в `stop-requested` возможен из-за ошибки, timeout, явного stop или
отмены родительского job. `cleaned` означает не «получили exit code», а:

- все прямые дети waited/reaped;
- process group получила escalation;
- pipe/file/PTY/pidfd descriptors закрыты;
- временные файлы удалены или сохранены в отчёте;
- capture readers остановлены;
- cgroup/session проверены или явно отмечены как оставшиеся;
- goroutines завершены.

## Общий результат

Каждый job возвращает структурированный результат:

```text
job_id
backend: process | pipeline | systemd | tmux | screen
started
finished
exit_code?
signal?
timed_out
cancelled
duration
pid/pgid/cgroup/session
stdout/stderr summary
resource snapshot: rss/pss/cpu/io
cleanup status
```

Нельзя скрывать cleanup failure за успешным exit code. Если команда завершилась,
но cgroup не опустела или PTY reader завис, результат должен быть `cleanup
warning/error`.

## Два режима вывода

### Stream mode

Обычный режим наследует stdout/stderr терминала или пишет в явный файл. Runtime
не копит весь output в памяти. Это default для `ssh`, `rsync`, `dmesg` и build.

### Capture mode

Только явный `capture` собирает stdout/stderr. У него обязательны:

- лимит bytes;
- лимит lines/records;
- политика при переполнении: stop, truncate с предупреждением или spool file;
- redaction для Secret и известных credential patterns;
- закрытие reader при timeout.

Не следует делать `CombinedOutput` универсальным backend’ом: большой или
зависший вывод тогда превращается в memory/pipe pressure.

## Timeout и отмена

Timeout имеет уровни:

```text
operation timeout
  → cancel context
  → SIGTERM / systemd stop / session stop
  → grace period
  → SIGKILL / cgroup kill
  → wait/reap
  → cleanup verification
```

Pipeline убивается как группа, systemd job — через unit/cgroup, tmux/screen —
через target session и её process group. Нельзя после timeout убить только
последний PID и считать job завершённой.

## Ресурсы

Снимок ресурсов делается:

- перед стартом, если доступно;
- после старта;
- после завершения;
- перед escalation при timeout.

Для Linux process lookup важны PID плюс start time: PID может быть переиспользован.
Для cgroup важнее `memory.current`, `memory.events`, `pids.current` и IO
счётчики всей job, а не RSS одного leader-процесса.

## Что видит пользователь

Не классы и не методы. Только операции, отражающие намерение:

```yesh
job = run command rsync -a source target
wait job within 1h
show job resources
stop job within 10s
```

Интерактивность — отдельная явная форма:

```yesh
session = tmux session "deploy"
session.start command ssh host shell
session.capture last 100 lines
```

Фоновый job не должен появляться из `&` или случайного detach. Он создаётся
только через явное `run`, `scope` или `session`, получает имя/owner/timeout и
попадает в cleanup registry.

## MVP-граница

Сначала реализовать общий internal `job` для обычного процесса и pipeline.
Затем подключить systemd scope. tmux/screen и memory sampling использовать как
следующие backend’ы, не смешивая их с базовым parser.
