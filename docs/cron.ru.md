# yesh из cron

Ночной скрипт должен завершаться без терминала, иметь определённые пути,
ограничение времени и защиту от одновременного повторного запуска.
Сейчас yesh запускается cron как обычный исполняемый файл; собственного
планировщика и встроенного редактирования crontab нет.

Пример `/opt/ops/service-check.yesh`:

```yesh
/usr/bin/test -r /etc/os-release
/bin/cat < /etc/os-release
```

Сравнение с Bash:

```bash
#!/bin/bash
set -euo pipefail
/usr/bin/test -r /etc/os-release
/bin/cat < /etc/os-release
```

Правило: cron вызывает абсолютный путь к yesh с абсолютным путём скрипта;
внутри скрипта файлы задаются абсолютно либо относительно заранее выбранного cwd.
Ошибка команды даёт CLI exit code 1 и останавливает следующие строки.

## Пример пользовательского crontab на Linux

```cron
SHELL=/bin/sh
PATH=/usr/bin:/bin
LC_ALL=C
MAILTO=ops@example.org
17 3 * * * /usr/bin/flock -n -E 75 /home/ops/.local/state/yesh/service-check.lock /usr/bin/timeout --signal=TERM --kill-after=10s 15m /usr/local/bin/yesh /opt/ops/service-check.yesh </dev/null >>/home/ops/.local/state/yesh/service-check.log 2>&1
```

Каталог `.local/state/yesh` заранее создаётся владельцем задания с mode 0700,
без symlink в пути; lock и log принадлежат этому пользователю. Log требует
отдельной ротации. Для `/etc/crontab` перед командой добавляется пользователь.
Не устанавливайте `SHELL=/usr/local/bin/yesh`: cron ожидает shell-интерфейс
`-c`, которого у yesh нет. `2>&1` здесь разбирает `/bin/sh` в crontab;
внутри yesh такой синтаксис пока не поддержан.

`flock -n -E 75` сразу возвращает 75 при занятом lock, а не копит очередь.
Lock удерживается и во время cleanup. Вариант argv используется без `-c`.
`timeout` задаёт общий бюджет скрипта; внутренний timeout yesh — пять минут
на команду, а не на все строки. CLI обрабатывает TERM/INT/HUP, останавливает
активную группу и ждёт детей. Внешний kill-after должен оставлять больше
двух секунд на внутреннюю эскалацию. После SIGKILL самого yesh cleanup
не гарантирован; detached descendants требуют cgroup/systemd backend.

Обычный `timeout` при превышении времени возвращает 124; это отличается от
exit code 1 самого yesh и 75 занятого lock. Cron сам по себе не превращает
ненулевой exit code в надёжный alert: нужен мониторинг. При полном редиректе
вывода MAILTO не получает содержимого log, а занятый lock может быть тихим.
Для NFS/CIFS следует отдельно проверить семантику lock.

## Проверка до установки задания

```bash
env -i HOME=/home/ops LOGNAME=ops PATH=/usr/bin:/bin LC_ALL=C \
    /usr/local/bin/yesh /opt/ops/service-check.yesh </dev/null
crontab -l
man 5 crontab
man 1 crontab
man 1 flock
man 1 timeout
```

В репозитории тестируется минимальное окружение, отсутствие stdin/TTY,
stdout/stderr, коды выхода, сигналы и конфликт lock. Рабочий crontab не меняется.
Для ручной установки нужен diff текущей таблицы и сохранение остальных строк;
`crontab -r` и слепая замена таблицы не входят в план MVP.

## Ошибки и различия cron

- `%` в командном поле cron обрабатывается ещё до shell, даже внутри кавычек;
  формат `date '+%F'` лучше перенести в файл yesh, а не писать в crontab.
- PATH в crontab задаётся целиком: `PATH=$HOME/bin:$PATH` не является
  обычным shell-присваиванием с подстановкой.
- Не полагайтесь на cwd, интерактивный profile, SSH agent, `sudo` prompt или TTY.
- Семантика timezone, DST, `@reboot`, пропущенных запусков и day-of-month /
  day-of-week зависит от cron реализации. Проверяйте локальные `cron(8)` /
  `crontab(5)`; не переносите молча поведение systemd timer.

Будущий typed backend должен сначала дать read-only list/show/validate/plan,
а затем атомарную замену только управляемого блока с проверкой пользователя,
lock, backup и конфликта изменившейся таблицы. Внутренний бесконечный scheduler
и автоматическое изменение системного crontab не нужны.
