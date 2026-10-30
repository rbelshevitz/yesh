# systemd и journalctl: план упрощайзера

Цель — не заменить `systemctl` и `journalctl`, а убрать повторяющийся
административный шум и сделать опасные операции заметными при чтении.

Полезная граница — небольшой типизированный адаптер над systemd, а не второй
язык конфигурации systemd. yesh должен коротко выражать частые вопросы:
«этот точный unit исправен?», «перезапуск завершился?», «покажи последние
ограниченные записи журнала?» — сохраняя в скрипте имя unit, manager, timeout и
изменяющее действие.

## Реализованный синтаксис

```yesh
service status api.service
service start api.service until active within 30s
service restart api.service until active within 30s
service wait api.service until active within 30s
journal api.service since 15m lines 100
service resources api.service
service status user api.service
journal user api.service since 15m lines 100
```

Короткие формы `service api`, `service.status api`, `service.restart api` и
`service.wait api running within 30s` — пока только эскизы дизайна и не входят
в поддерживаемый синтаксис. Выше показан текущий интерфейс.

`service status`, `service start UNIT until active within DURATION`,
`service restart UNIT until active within DURATION`,
`service wait UNIT until active within DURATION` и
ограниченный `journal UNIT since DURATION lines N` уже реализованы. Перезапуск
использует единый конечный бюджет клиента и опрашивает состояние точной службы
до требуемого состояния или крайнего срока; `systemctl --wait` не используется, потому что
для долгоживущей службы он может ждать её завершения. Журнал выдаёт строки JSON, ограничен
четырьмя MiB и отклоняет диагностику в stderr, чтобы предупреждение о правах
или частичный результат не выглядели успехом. Фильтры загрузки и чтение потока
пока остаются целевым дизайном; ограниченное ожидание уже реализовано.

`service resources UNIT` — снимок службы Linux и всей её cgroup только для чтения:
`ControlGroup`, главный PID, число процессов, суммарный RSS, доступные
счётчики памяти, задач и CPU, OOM-события и таблица процессов. Главный PID не
считается всей службой: дочерние процессы берутся из cgroup и её подгрупп.
Недоступное поле выводится как `unavailable`, а исчезнувший процесс может не
попасть в моментальный снимок. Операция не посылает сигналов и не меняет
лимиты. Она поддерживается только на Linux с доступным systemd и cgroup v2;
это снимок состояния, а не доказательство будущего поведения службы.

`DURATION` пока использует форму длительности Go, принятую yesh: например,
`300ms`, `2s`, `15m` или `1h30m`. Полный синтаксис `systemd.time` намеренно
не принимается.

Перед unit можно указать manager: `system` или `user`. Если профиль опущен,
используется системный manager для совместимости со старым синтаксисом.
Профиль `user` добавляет `--user` и для `systemctl`, и для `journalctl`;
это не означает наличие persistent user journal или достаточных прав чтения.

`service wait` ничего не запускает и не перезапускает: он сразу читает
состояние, затем опрашивает его до конечного срока. При timeout выводится
последний `ActiveState`/`SubState`. Это состояние manager, а не readiness
приложения. Для `Type=oneshot` postcondition `active` имеет ожидаемый смысл
только при `RemainAfterExit=yes`; без него успешный oneshot обычно становится
`inactive/dead`, и yesh не угадывает это как успех. После `start` или `restart`
такое terminal-состояние сразу считается невозможным postcondition, без
ожидания всего timeout.
Состояния `failed`, `not-found` и `masked` сообщаются сразу, без ожидания
всего timeout; переходные `inactive`, `activating` и `deactivating` могут
измениться внешним актором и поэтому опрашиваются.

## Что именно упрощается

Упрощайзер должен убирать только повторение и опасные значения по умолчанию:

| Повторяющаяся работа с systemd | Упрощение в yesh |
| --- | --- |
| `systemctl show` и ручной список свойств | один ограниченный снимок службы с именованными полями |
| start/restart и отдельный цикл опроса | одно действие с обязательным postcondition и timeout |
| разбор текста `systemctl status` | структурированное состояние по allowlist свойств |
| `journalctl` с pager и неограниченным выводом | ограниченные записи, явный интервал, без pager |
| разный вызов system/user manager | явный выбор manager в операции |

Адаптер не должен скрывать `sudo`, выбор unit, изменения или ошибки. Он не
принимает glob и не действует молча на нескольких unit. `daemon-reload`,
`reload`, `restart`, `enable`, `disable`, `mask` и `unmask` — разные операции и
должны оставаться разными в yesh.

## Опасные нюансы systemd

Это границы дизайна, а не мелкие детали реализации:

- Alias — допустимая явная цель systemd, но `systemctl show` может вернуть
  canonical `Id`. Текущий MVP явно отклоняет такой mismatch и сообщает, что
  aliases не поддержаны; принятие alias требует отдельной политики identity.
- `foo@.service` — template, а `foo@blue.service` — конкретный instance.
  Первый отклоняется как цель, а concrete instance остаётся явным именем.
- `Type=oneshot` без `RemainAfterExit=yes` после успешного завершения переходит
  в inactive/dead. Поэтому `active` не означает успех для любого типа unit.
- `systemctl restart` ждёт job manager, если не указан `--no-block`, но это не
  доказательство готовности приложения. После него могут потребоваться
  `Result`, exit status процесса, readiness probe или предметный postcondition.
- `--system` и `--user` обращаются к разным manager. Доступ к journal также
  зависит от прав, а `--user` ограничен при отключённом persistent journal.
- `journalctl --unit=foo.slice` включает записи дочерних unit. Это нельзя
  описывать как фильтр одной службы.
- Пустой journal — нормальный результат. Его нужно отличать от отказа доступа,
  недоступного manager, ротации старых записей и повреждённого/частичного JSON.

## Минимальный полезный интерфейс

Первый слой должен быть небольшим:

```yesh
service status api.service
service start api.service until active within 30s
service restart api.service until active within 30s
service wait api.service until active within 30s
journal api.service since 15m lines 100
```

`service status` должен различать как минимум `active`, `inactive`, `failed`,
`activating`, `deactivating`, `masked`, `not-found`, недоступный manager и
отказ в доступе. `service restart` должен сообщать и результат команды, и
проверенное итоговое состояние. Успешный возврат клиента `systemctl` сам по
себе не доказывает, что служба достигла нужного состояния.

Грамматика этих четырёх форм уже проверяется parser/runtime-тестами. Важнее
семантическое правило:
каждая изменяющая операция со службой имеет явное postcondition либо явно
возвращает только результат manager; timeout никогда не выдаётся за доказанную
остановку.

## Карта примеров и fixture

| Нюанс | Пример | Что доказывает fixture |
| --- | --- | --- |
| точная unit и фиксированные поля | `examples/service-status.yesh` | allowlist, canonical Id, отсутствие изменения |
| restart не равен readiness | `examples/service-restart.yesh` (fixture-backed) | отказ postcondition останавливает скрипт |
| конечный polling | `examples/service-wait.yesh` | переход в active, timeout, быстрый failed-state |
| ограниченный journal | `examples/service-journal.yesh` | JSON, ошибка stderr, успешный пустой результат |
| ресурсный снимок службы | `examples/service-resources.yesh` | связь cgroup с процессами и ограниченные счётчики |

Для проверки настоящего systemd есть отдельный opt-in-скрипт
[`scripts/yesh-fixture-test.sh`](../scripts/yesh-fixture-test.sh). Он создаёт
уникальные временные юниты только в `/run/systemd/system`, проверяет реальные
`status`, `restart` и `wait`, отдельно подтверждает `oneshot` без
`RemainAfterExit` и timeout во время `activating`, а затем удаляет только свои
файлы через `trap`. Скрипт требует интерактивный `sudo` и не должен запускаться
на production-службах.

Примеры намеренно подробно прокомментированы. У каждого есть Bash-сравнение,
а practical fixture запускает сам файл с fake `systemctl`/`journalctl` и
проверяет argv, вывод, порядок, остановку после ошибки и `--plan` без
обращения к настоящему manager.

## Masked unit — это состояние, а не ошибка parser-а

`LoadState=masked` — допустимое наблюдение. `service status` его показывает,
а `service wait` сразу завершается ошибкой и не тратит весь timeout. yesh не
добавляет скрытый `systemctl unmask`; если unmask появится, это должна быть
отдельная явная изменяющая операция.

## Текущая граница и следующий поиск

В MVP уже реализованы `service status` только для чтения, ограниченное
`service wait`, start и restart с требуемым состоянием, ограниченный журнал в
JSON и `service resources` для снимка cgroup. Они используют фиксированный
allowlist `systemctl show`, точную identity unit, профиль system/user, конечные
сроки и прозрачный argv в `--plan`.

Отдельно остаются: stop/reload, queued job и readiness после start/restart,
cursor/boot и allowlist полей journal, политика ротации/cursor, follow/streaming
и backend transient
service через `systemd-run`. Текущий MVP этого не обещает.

Эти операции должны быть типизированными командами runtime с тем же прозрачным
планом `argv`, что и внешние команды, а не текстовыми макросами. Так `--plan`
и диагностика сохранят одну семантику.

Текущая граница subprocess — обычные команды, которые можно увидеть в плане:

```text
systemctl show --no-pager --property=Id,LoadState,ActiveState,SubState api.service
systemctl restart api.service
journalctl --no-pager --output=json --unit api.service --since -15m --lines 100
```

`service restart` выполняет вторую команду, а затем снова читает первый снимок
для проверки postcondition. Точные argv и профиль manager остаются видны в
`--plan`; это граница текущей реализации, а не обещание навсегда оставить все
будущие операции systemd на subprocess.

## Защита от ошибок

- имя unit проверяется как одно значение, без скрытого shell-разбора;
- `restart`, `stop`, `disable` и `mask` явно считаются изменяющими операциями;
- `--plan` показывает команду и unit, но не меняет состояние;
- `wait` имеет обязательный timeout и конечное число проверок;
- `journal` ограничивает время, число строк и отключает pager;
- не добавлять `sudo` автоматически: право видно в исходнике;
- диагностика сохраняет exit status, signal, timeout и stderr;
- не читать бесконечный journal-поток без явного `follow`-режима;
- `follow` в MVP запрещён, пока нет отдельной модели отмены и закрытия TTY.

## Принятое решение

`service` и `journal` — встроенные типизированные операции, а не текстовые
макросы над `systemctl` и `journalctl`. Позднее transport можно заменить на
native D-Bus, но identity, состояния, timeout, диагностика и postcondition
должны остаться наблюдаемыми и эквивалентными.
