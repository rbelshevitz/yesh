# POSIX-нюансы runtime

yesh сохраняет Unix-модель процессов, но не обязан копировать все правила
POSIX shell. Отличия должны быть сознательными и описанными.

## Pipeline

Каждая стадия pipeline — отдельный процесс. Runtime обязан создать и соединить
pipe между соседними стадиями, закрыть в родителе лишние концы после запуска,
дождаться всех стадий и сохранить результат каждой стадии.

Если лишний процесс держит write-end открытым, читатель никогда не увидит EOF.
Если читатель завершился, writer может получить `SIGPIPE`/`EPIPE`; это нормальная
часть Unix pipeline, а не обязательно ошибка самого yesh. [Linux pipe(7)](https://man7.org/linux/man-pages/man7/pipe.7.html)

POSIX shell обычно возвращает status последней команды pipeline. yesh намеренно
выбирает более безопасное поведение: pipeline успешен только если успешны все
стадии. Это аналог `pipefail`, но встроенный и не отключаемый флагом.

## Timeout и завершение

Timeout одной команды недостаточен для pipeline: убийство одного процесса может
оставить его детей работать. На POSIX все стадии job должны получать общую
process group; при отмене runtime посылает сигнал группе, затем дожидается всех
детей и через короткий grace period использует `SIGKILL`. Группа настраивается
последовательно: первая реально запущенная стадия становится leader, следующие
присоединяются к её PGID. Нельзя назначать PGID всем стадиям до запуска первой.

Отмена имеет два уровня: `SIGTERM` сразу после timeout/cancel и `SIGKILL` после
grace period. Эскалация живёт только во время команды и закрывается при любом
пути возврата, чтобы safety-логика сама не создавала goroutine leak.

Process groups также нужны, чтобы Ctrl-C попадал во весь foreground pipeline,
а не только в одну стадию. [Linux process groups](https://man7.org/linux/man-pages/man7/credentials.7.html)

## TTY и job control

Для файловых DevOps-скриптов достаточно наследовать stdin/stdout/stderr.
Полноценный интерактивный shell дополнительно требует controlling terminal,
foreground process group, сигналы `SIGINT`/`SIGTSTP`/`SIGCONT`/`SIGWINCH`,
остановку jobs и PTY для программ, меняющих поведение без terminal.

Это отдельный этап. До его реализации yesh не обещает Bash-совместимое
поведение `fg`, `bg` и Ctrl-Z.

## Redirections

Redirection — часть AST, а не текстовая вставка. Runtime явно определяет, кто
открывает файл, с какими flags и mode, в каком порядке применяются
stdin/stdout/stderr, кто получает ошибку открытия и когда descriptor закрывается.

Нельзя автоматически делать redirection привилегированным из-за `sudo` в
команде. `sudo command > file` и `echo data | sudo tee file` — разные операции.

## Exit status и signals

Результат процесса хранит как минимум:

```text
started
exit_code
signal
timed_out
stderr
duration
```

Нельзя превращать убийство сигналом в обычный exit code без потери информации.

## Environment и executable lookup

Окружение передаётся явно и не должно случайно смешиваться с переменными yesh.
`SSH_AUTH_SOCK`, `PATH`, `HOME` и `TERM` имеют операционный смысл; `sudo` может
удалить часть из них своей политикой.

Внешняя команда запускается как `argv`, без системного shell, globbing и
word-splitting. `PATH` lookup должен быть диагностируемым; неожиданный
`./program` не должен незаметно выигрывать у системной команды.

## Что yesh сознательно меняет

- неявное word splitting отсутствует;
- pipeline failure проверяется по всем стадиям;
- бесконечные loops и background jobs запрещены в базовом режиме;
- command substitution и nested shell запрещены;
- secrets не становятся частью argv или логов;
- обычный скрипт не требует PTY.
