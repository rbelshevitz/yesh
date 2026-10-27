# Syslog, journald и boot log

## Не смешивать источники

В Linux «лог загрузки» может означать несколько вещей:

- kernel ring buffer, доступный через `dmesg`;
- systemd journal текущего или предыдущего boot;
- сообщения journald с `_TRANSPORT=kernel`;
- классический syslog/rsyslog в `/var/log/syslog` или `/var/log/messages`;
- отдельный `/var/log/boot.log`, если его создаёт distro/service.

yesh должен показывать источник в отчёте:

```text
source: systemd-journal
boot: current
storage: persistent
records: 184
```

Если journal volatile, старые boot могут отсутствовать. `Storage=volatile`
хранит данные только в `/run/log/journal`, а persistent-журнал обычно находится
в `/var/log/journal`; `journalctl --flush` переносит ранние записи туда, когда
это возможно. [journald.conf](https://www.freedesktop.org/software/systemd/man/262/journald%40.conf.html)

## Целевой интерфейс yesh

```yesh
journal boot current since 10m priority warning
journal unit sshd since 1h
journal kernel boot previous
syslog file /var/log/syslog since 30m
bootlog show current
```

Это не обязано быть пятью разными реализациями. Внутри нужен `LogQuery`:

```text
source: journal | syslog-file | boot-file | kernel-ring
boot: current | previous | id
unit / identifier / facility / priority
since / until
grep regex
limit
format: text | json
```

Но источник должен быть явным в AST или хотя бы в результате autodetect.
Порядок fallback для `bootlog show`:

1. systemd journal с boot selector;
2. kernel messages из journal;
3. `/var/log/boot.log`;
4. distro syslog-файл с пометкой о неполной семантике.

«Не найдено» и «источник не хранит историю» — разные результаты.

## Backend systemd

Для первой версии безопаснее вызывать `journalctl` через argv:

```text
journalctl --system --boot=0 --since=10m --priority=warning \
  --no-pager --output=json-seq
```

JSON/JSON-seq позволяет получить trusted fields: `_PID`, `_EXE`,
`_SYSTEMD_UNIT`, `_BOOT_ID`, `_TRANSPORT`, `PRIORITY`, `MESSAGE`, вместо
разбора красивого terminal output. `journalctl` поддерживает выбор boot,
priority, unit, kernel-only messages и regex-фильтр MESSAGE. [journalctl](https://www.freedesktop.org/software/systemd/man/255/journalctl.html),
[journal fields](https://www.freedesktop.org/software/systemd/man/257/systemd.journal-fields.html)

Позже возможен native backend через `go-systemd`/D-Bus или `sdjournal`, но
`sdjournal` требует CGO. Это Linux-specific оптимизация, не MVP.

## Backend classic syslog

Не обещать единую структуру там, где её нет. Для `/var/log/syslog`,
`/var/log/messages` и `/var/log/boot.log` MVP возвращает records:

```text
source, path, timestamp?, hostname?, identifier?, raw_message, line
```

RFC 3164/5424 parsing — отдельный режим. Сначала безопасное чтение snapshot:

- открыть файл read-only;
- получить размер и читать до зафиксированного конца;
- ограничить bytes/lines;
- не следовать за logrotate symlink без повторной проверки;
- не запускать `tail -f`;
- явно показать, что строки могут быть потеряны между rotation и чтением.

Rsyslog обычно пишет `/var/log/syslog` через конфигурацию `/etc/rsyslog.conf`
и `/etc/rsyslog.d/*.conf`, но путь distro-specific. [rsyslog configuration](https://docs.rsyslog.com/doc/getting_started/basic_configuration.html)

## Диагностика boot/device

```yesh
diagnose boot current:
    journal kernel priority warning
    journal priority err
    device /dev/nvme0n1
    module nvme
```

Отчёт должен разделять:

1. journal недоступен из-за прав;
2. journal доступен, но boot history не сохраняется;
3. kernel видел устройство, но udev не создал node;
4. udev создал node, но driver/module не загрузился;
5. driver загрузился, но есть firmware/I/O errors;
6. filesystem найден, но mount не выполнен.

## Безопасность и лимиты

- `journalctl -f`, `dmesg --follow` и `udevadm monitor` не входят в MVP;
- `--clear`, `--read-clear`, vacuum и rotation не являются read-only query;
- исторические boot выбираются по ID/offset, а не по угадыванию даты;
- вывод ограничивается bytes/records, чтобы не съесть память;
- сообщения могут содержать credentials: отчёт не должен автоматически
  пересылаться на удалённый host или попадать в error string целиком;
- `--plan` показывает фильтры, источник и лимиты, но не печатает весь журнал.

## logrotate и опасные места

`logrotate` — это не просто переименование файла. Он ведёт state file,
проверяет условия времени/размера, создаёт новый файл, меняет ownership/mode,
вызывает hooks и может сжимать или удалять старые копии.

Типовая безопасная схема:

```text
preflight config → lock state → check source/owner/mode
→ rename old log → create new log → reopen daemon
→ compress delayed copy → verify → prune by retention
```

### `copytruncate` — исключение

`copytruncate` оставляет inode на месте и обнуляет файл после копирования.
Процесс продолжает писать в тот же fd, но записи, появившиеся между copy и
truncate, могут потеряться. Использовать только когда daemon невозможно
попросить reopen; в plan показывать риск и окно операции. [logrotate(8)](https://www.man7.org/linux/man-pages/man8/logrotate.8.html)

Предпочтительно:

```text
rename → create с mode/owner → signal/reopen daemon → verify new inode
```

Для systemd-сервисов yesh должен использовать явный `systemctl reload` или
документированный signal/reopen action, а не выполнять произвольный
`postrotate` shell script из чужой конфигурации.

### Остальные ловушки

- `sharedscripts` меняет число запусков hooks и меняет их аргументы;
- ошибка hook может остановить оставшуюся часть rotation;
- `postrotate` выполняется до compression;
- `olddir` обычно должен быть на том же filesystem, иначе нужны copy/renamecopy;
- `create` определяет mode/owner нового файла; `su` меняет пользователя rotation;
- `delaycompress` оставляет последнюю rotated copy несжатой для daemon’ов,
  которые ещё держат старый fd;
- `rotate 0` удаляет старые версии вместо хранения;
- `-f/--force` нельзя добавлять автоматически;
- state file и lock должны быть видны в плане;
- symlink, hardlink, неожиданный owner или каталог под контролем непривилегированного
  пользователя — остановка, а не «починка».

### Journald не обслуживается logrotate

Файлы `/var/log/journal/*.journal` управляются journald. Для них нужны
`SystemMaxUse=`, `SystemKeepFree=`, `SystemMaxFiles=`, `--rotate` и
`--vacuum-*`; active journal files могут остаться после vacuum, поэтому
результат нужно проверять по `--disk-usage`. [journald limits](https://www.freedesktop.org/software/systemd/man/260/journald.conf.d.html),
[journalctl vacuum](https://www.freedesktop.org/software/systemd/man/255/journalctl.html)

### Целевой yesh-интерфейс

```yesh
rotation = log policy /var/log/myapp/app.log
    keep 14
    when size 100MiB
    compress delayed
    reopen service myapp

logrotate plan rotation
logrotate apply rotation
```

`plan` только читает config/state и показывает будущие rename/create/delete.
`apply` требует lock, проверяет filesystem/owner/mode и не разрешает
`copytruncate` без явного `allow loss risk`.
