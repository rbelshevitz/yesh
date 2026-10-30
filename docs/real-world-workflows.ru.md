# Длинные практические сценарии yesh

Этот документ отвечает на практический вопрос: можно ли на yesh писать не
только короткие примеры синтаксиса, но и осмысленные Linux-runbook’и длиной
примерно 100–200 строк. Сценарий здесь — не порт Bash один к одному, а
самостоятельная адаптация операции с понятными входами, побочными эффектами,
проверками и путём отказа.

## Как читать этот корпус

Каждый сценарий должен иметь:

1. disposable fixture или явно read-only режим;
2. положительный прогон и отрицательный прогон;
3. `--plan`, в котором видны раскрытые `argv` и не меняется fixture;
4. проверку результата, а не только код `0`;
5. короткое Bash-сравнение и объяснение исчезнувшей ловушки;
6. границу того, что проверка не доказывает на production-хосте.

Целевой размер — 100–200 строк yesh после удаления комментариев, которые
описывают fixture и ограничения. Это не требование искусственно раздувать
пример: если сценарий получается короче, его следует объединять с соседними
фазами только тогда, когда они составляют один операторский workflow.

## Корпус первого прохода

| ID | Runbook | Основные фазы | Статус |
| --- | --- | --- | --- |
| B1 | Инкрементальный backup и restore drill | preflight → lock → rsync → publish → retention plan → restore → verify | минимальный RWE пройден |
| B2 | Ротация backup с quarantine | configtest → test mode → lock → rotation → quarantine → restore | RWE пройден |
| B3 | Публикация конфигурации и проверка службы | validate → stage → publish → restart → postcondition → journal | RWE пройден |
| B4 | Безопасный deploy одного release | preflight → upload → symlink/publish → service restart → rollback check | RWE пройден |
| B5 | Диагностика failed service | status → bounded journal → process snapshot → storage → report | RWE пройден |
| B6 | Проверка backup-диска перед записью | fstab/UUID → mount → filesystem → free space → rsync preview | минимальный RWE пройден |
| B7 | Remote pull с проверкой SSH | host-key policy → SSH health → remote source → rsync → checksum/restore | RWE пройден |
| B8 | Восстановление выбранной версии | snapshot selection → empty target → rsync restore → manifest/checksum → report | RWE пройден |
| B9 | Read-only incident bundle | service → journal → processes → mounts → network → archive | проект корпуса |
| B10 | Проверка initrd перед перезагрузкой | kernel/module manifest → initrd contents → boot config → report | runnable составными частями |
| B11 | Проверка сетевой зависимости backup | маршрут → DNS → TCP/SSH → удалённый путь → пробный запуск | минимальный RWE пройден |
| B12 | Ночной cron-runbook без изменения crontab | environment → lock → deadline → backup → result log → alert decision | минимальный RWE пройден |

Первый проход не означает, что все двенадцать нужно реализовать сразу. Он
нужен, чтобы одинаково сравнивать сценарии и увидеть повторяющиеся пробелы
языка. B1, B3 и B5 — минимальная тройка, на которой уже проверяется ценность
yesh как читаемого runbook’а, а не как оболочки вокруг `rsync`.

## Карточки сценариев

### B1. Инкрементальный backup и restore drill

**Операторский результат:** новый снимок появляется только после успешного
`rsync`; неизменённые файлы используют hard link; выбранный снимок можно
восстановить в пустой каталог и проверить по содержимому, типам и checksum.

**Фазы:** проверить source, destination filesystem, свободное место и lock;
создать private staging; вызвать видимый `rsync` с явным `--link-dest`; сделать
same-filesystem `publish`; показать retention plan; восстановить снимок
отдельным `rsync`; проверить результат.

**Bash-боль:** временные имена, `set -e` вокруг pipeline, случайный
`--delete`, публикация частичного дерева, потеря lock при раннем выходе и
неочевидное различие локального пути и remote rsync source.

**Отказы:** отсутствующий source, занятый lock, недостаток места, код rsync
23, существующий final path, повреждённый restore target. Каждый отказ должен
оставить предыдущий опубликованный snapshot неизменным.

**Граница:** это не согласованный снимок изменяющейся базы данных и не гарантия
сохранности после потери питания. Engine backup остаётся `rsync`.

Минимальный RWE B1 пройден на disposable mounted ext4: два запуска реального
yesh/rsync, `--plan`, `--link-dest`, restore и missing-source code 23 без
публикации нового snapshot. Crash-durability и полный failure matrix остаются
отдельными границами.

### B2. Ротация backup с quarantine

**Операторский результат:** после проверки конфигурации старые завершённые
снимки под lock перемещаются в quarantine на той же файловой системе; удаление
quarantine — отдельное явное действие.

**Почему это длинный тест:** здесь есть политика выбора snapshot, проверка
имён и symlink, конфликт lock, два разных шага применения и восстановление
старой версии до физической очистки.

**Отказы:** плохой `retain`, неожиданный каталог, cross-filesystem rename,
занятый lock, ошибка `rsnapshot configtest` или test mode. Нельзя считать
частично перемещённую ротацию успешной.

**Открытый вопрос реализации:** безопасная операция quarantine ещё не должна
появляться в языке только из-за этого сценария. Сначала нужен runnable fixture
и подтверждение, что обычные `mv` с явным preflight не дают достаточно
читаемого контракта в B2 и ещё одном workflow.

RWE B2 пройден на disposable mounted ext4: две реальные rotation через
`rsnapshot-apply.yesh`, `--plan`, same-filesystem quarantine, restore старой
версии и cleanup lock/mount без busy. Встроенная quarantine-операция yesh ещё
не создавалась.

### B3. Публикация конфигурации и проверка службы

**Операторский результат:** staged-конфигурация проходит внешний validator,
публикуется атомарно, служба перезапускается с конечным timeout и считается
успешной только после postcondition; при отказе старая конфигурация остаётся
доступной для rollback.

**Фазы:** проверить владельца и mode; вызвать `nginx -t`, `sshd -t` или
аналогичный validator в disposable fixture; установить файл во временное имя;
опубликовать; `service restart`; `service status`/`wait`; ограниченно прочитать
journal; при неуспехе показать точку восстановления.

**Bash-боль:** command substitution для извлечения статуса, ручное quoting
путей, `&&`-цепочка, которая теряет причину отказа, и ложный успех после
принятия restart job без проверки готовности.

**Отказы:** validator не принимает файл, target не является ожидаемым файлом,
служба failed/activating, timeout, отказ доступа. При ошибке validator старый
файл не меняется.

**Граница:** yesh не угадывает application readiness и не добавляет `sudo`.
Конкретный validator и postcondition принадлежат оператору и unit.

RWE B3 пройден на disposable Debian systemd unit: validator `grep`,
`--plan` без записи, staging через `install`, same-filesystem `publish`,
typed restart/status с bounded postcondition, bounded JSON journal и отказ
невалидной конфигурации до изменения active-файла. Fixture —
`scripts/yesh-config-systemd-rwe.sh`, пример —
`examples/config-systemd-rwe.yesh`.

### B4. Безопасный deploy одного release

**Операторский результат:** release собирается или проверяется внешними
командами, передаётся на host, становится видимым одной публикацией, затем
служба проверяется; старый release можно назвать и вернуть без поиска по
непредсказуемым glob’ам.

**Отказы:** неполный upload, неверная версия, занятый lock, service failure,
отсутствующий rollback target. Новая ссылка не должна появиться до успешной
проверки полного release.

**Граница реализации:** `ssh`/`scp` остаются внешними командами; отсутствие
поддержки `cd` и контролируемого environment нужно явно учитывать в fixture,
а не обходить скрытым shell wrapper’ом.

RWE B4 пройден из operator workspace на Debian target: строгая SSH host-key
policy, remote staging через `rsync --delay-updates`, публикация release,
атомарное переключение `current` через временную symlink и `mv -T`, rollback,
`--plan` без remote changes и missing-source negative. Fixture —
`scripts/yesh-deploy-release-rwe.sh`, пример —
`examples/deploy-release-rwe.yesh`. Service restart намеренно не включён в
этот fixture: он отдельно доказан B3 на disposable systemd unit.

### B5. Диагностика failed service

**Операторский результат:** один read-only отчёт связывает состояние unit,
ограниченный journal, процессы, mount’ы и свободное место; отчёт не меняет
службу и не сигналит чужим PID.

**Фазы:** `service status`; `journal` с конечным объёмом; typed process
snapshot; `findmnt`/`df`; сохранить stdout/stderr отдельных источников;
собрать архив отчёта и вывести краткое заключение.

**Bash-боль:** `ps | grep`, потеря exit code pipeline, неограниченный `journalctl`,
смешивание диагностического stderr с данными и гонки при повторном чтении
процессов.

**Отказы:** unit отсутствует, journal недоступен, процесс исчез между снимками,
нет места для bundle. Частичный отчёт должен быть явно помечен неполным.

**Граница:** snapshot не является полной forensic-копией и не доказывает
причину инцидента; он фиксирует наблюдаемые признаки.

RWE B5 пройден как read-only композиция на Linux/systemd: `service status`,
bounded JSON journal, typed process snapshot, `findmnt`/`df`, раздельные
stdout/stderr, `tar`-архив отчёта и missing-unit negative. `--plan` не создаёт
файлы отчёта. Новый встроенный bundle/archive API не добавлялся: архив
остаётся видимой внешней командой. Fixture —
`scripts/yesh-incident-report-rwe.sh`, пример —
`examples/incident-report-rwe.yesh`.

### B6. Проверка backup-диска перед записью

**Операторский результат:** backup запускается только если путь соответствует
ожидаемому UUID/PARTUUID, является точным mountpoint и имеет достаточный запас
места.

**Отказы:** незамонтированный путь, другой UUID, readonly filesystem,
недостаток места, поддерево вместо mountpoint. `rsync` в этих случаях не
запускается.

**Польза для языка:** typed storage-проверки убирают ручное сопоставление
выводов `findmnt`, `lsblk`, `blkid` и арифметику над локализованным `df`.

RWE минимального среза пройден на disposable loop/ext4 mount: реальный yesh,
positive rsync, `--plan` без записи, wrong-UUID, readonly, unmounted и
low-space negative; при отказе опубликованный файл сохраняется. Полный
production-like failure matrix и отдельная VM остаются следующим расширением.

### B7. Remote pull с проверкой SSH

**Операторский результат:** удалённый источник проверен отдельным SSH health
шагом с явной host-key policy, затем выполняется `rsync` на локальный
подготовленный destination и проверяется восстановлением.

**Отказы:** DNS/route/TCP failure, host key mismatch, auth failure, remote
source missing, partial rsync. Нельзя включать `StrictHostKeyChecking=no` как
«исправление».

**Граница:** yesh показывает argv и порядок, но не доказывает безопасность
удалённого узла или корректность его файловой системы.

RWE B7 пройден на отдельном Debian: `BatchMode=yes`, `ConnectTimeout=5`,
`StrictHostKeyChecking=yes`, SSH/SCP checksum, два remote pull с
`--link-dest`, отказ чтения удалённого файла без публикации нового snapshot,
повтор после восстановления доступа и missing-source negative. Тесты —
`TestPOSIXRealSSHAndSCP`, `TestPOSIXRealRsyncPullAndIncremental` и
`TestPOSIXRealRsyncRemoteFailureDoesNotPublish`.

### B8. Восстановление выбранной версии

**Операторский результат:** оператор явно выбирает завершённый локальный
snapshot, а `rsync` восстанавливает его только в существующий пустой каталог.
После копирования проверяются содержимое и checksum; target не очищается
неявно.

**Отказы:** отсутствующий snapshot, symlink вместо snapshot и непустой target.
Каждый отказ оставляет уже восстановленные или исходные данные неизменными.

RWE B8 пройден на Debian и локально: реальный yesh/rsync, `--plan` без записи,
положительное восстановление, проверка checksum, symlink, непустой target и
отсутствующий source как отрицательные сценарии. Fixture —
`scripts/yesh-selected-restore-rwe.sh`, пример —
`examples/rsync-selected-restore.yesh`.

### B9. Read-only incident bundle

**Операторский результат:** ограниченный архив для передачи в тикет содержит
версии, service/journal, process, storage и network diagnostics с раздельными
потоками данных и ошибок.

**Отказы:** любой источник недоступен или превышен размер/время. Bundle должен
содержать статус каждого этапа, а не выдавать неполный архив за полный.

**Потенциальный повторяющийся пробел:** если для B5 и B9 потребуется одна и
та же безопасная модель bounded report/state, это кандидат на общую возможность;
до этого не добавлять «report DSL».

### B10. Проверка initrd перед перезагрузкой

**Операторский результат:** read-only проверка подтверждает наличие ожидаемого
модуля и конфигурации в initrd, соответствие kernel artefact и отсутствие
неожиданной замены до отдельного решения о reboot.

**Отказы:** отсутствующий модуль, неверный формат initrd, несоответствие
manifest, неизвестный compression tool. Никаких `modprobe`, reboot или записи
в `/boot` в обычном fixture.

**Граница:** наличие `.ko` в искусственном образе не доказывает привязку
устройства и успешную загрузку ОС.

### B11. Проверка сетевой зависимости backup

**Операторский результат:** до backup видны маршрут, DNS, TCP/SSH policy и
доступность remote path; затем выполняется только разрешённая операция.

**Отказы:** неправильный маршрут, DNS mismatch, закрытый порт, host-key
ошибка, remote path missing. Прогон должен различать network preflight и
ошибку самого `rsync`.

Первый составной пример — `examples/rsync-network-preflight.yesh`: он оставляет
`getent`, `ip route get`, SSH-политику и `rsync --dry-run` видимыми внешними
командами. `--plan` показывает обе ветви отказа; положительный пробный запуск
проверен с рабочего места оператора на Debian. Последовательность отказов с
проверкой порядка и отсутствия запасного обхода находится в
`scripts/yesh-rsync-network-preflight-rwe.sh`. Минимальный RWE с публикацией
снимка, `--plan` и отсутствующим удалённым путём пройден в
`scripts/yesh-rsync-networked-incremental-rwe.sh`; обрыв посередине передачи
и полная матрица частичного `rsync` ещё не заявляются.

### B12. Ночной cron-runbook без изменения crontab

**Операторский результат:** запуск из cron имеет абсолютные пути, явное
окружение, lock, конечный бюджет, журнал и однозначный код для alerting.

**Отказы:** уже запущенный экземпляр, отсутствующий environment, timeout,
частичный backup, недостаток места. Сам crontab в сценарии не изменяется.

Минимальный RWE B12 пройден на disposable каталоге: `env -i`, внешние
`flock`/`timeout`, внутренний yesh lock, положительная публикация, занятый
внешний lock, отсутствующий source и timeout без публикации. Реальный cron
daemon и изменение crontab намеренно не используются. Fixture —
`scripts/yesh-cron-backup-rwe.sh`, пример — `examples/cron-backup-rwe.yesh`.

**Открытый вопрос:** общий durable state/resume намеренно не добавляется до
появления двух сценариев, где одного lock и журналирования недостаточно.

## Порядок реализации

1. Довести B1/B11 до общей failure matrix: timeout, partial transfer,
   отсутствие публикации и безопасный повторный запуск.
2. Закрыть B9 как read-only incident bundle, не добавляя общий report DSL.
3. Продолжить сетевой слой: neighbours и listeners через внешние read-only
   `ip`/`ss`, затем проверить, нужны ли typed значения в ещё одном workflow.
4. После повторения одной проблемы в двух независимых сценариях решать,
   нужен ли новый runtime-интерфейс; внешние `rsync`, `ip`, `ss`, `ssh` и
   `tar` не заменять встроенными копиями.

## Что будет считаться ответом на вопрос «yesh подходит?»

yesh подходит для длинного сценария, если читатель может сверху вниз назвать
точный `argv`, порядок побочных эффектов, условие отказа и сохранённое старое
состояние. Небольшое число строк само по себе ничего не доказывает. Если
длинный сценарий требует скрытого shell-кода, неявного word splitting,
неограниченного ожидания или незаметного destructive default, это пробел
контракта, а не повод объявлять адаптацию успешной.

