# Пакеты: APT, dpkg, RPM, alien и snapd

Цель — проверить установленную версию, понять изменение до установки и не
оставить сервер с незавершённой транзакцией. Пока это внешние команды;
типизированный package API ниже — план, не реализованный синтаксис.

## Уже работающий read-only сценарий

```yesh
# Debian / Ubuntu: сведения о конкретном установленном пакете.
dpkg-query --show --showformat='${binary:Package}\t${Version}\t${db:Status-Status}\n' dpkg
apt-cache policy dpkg
```

Сравнение с Bash:

```bash
set -euo pipefail
dpkg-query --show --showformat='${binary:Package}\t${Version}\t${db:Status-Status}\n' dpkg
apt-cache policy dpkg
```

Правило: backend выбирается явно, запрос использует известные поля и точное
имя пакета; отсутствие пакета отличается от сбоя базы, доступа или daemon.
`${...}` здесь — формат самой `dpkg-query`, а не подстановка yesh.

Ошибка: `dpkg-query --show no-such-package` возвращает ненулевой status;
обычная команда останавливает скрипт, условие `if` получает false. До typed
API условие не отличает отсутствие пакета от другого обычного ненулевого
exit code; для диагностики сохраняйте stderr и не скрывайте его через `if`.

## Backend и источники

| Backend | Сначала read-only / preflight | Явное изменение, отдельный этап | Man / официальный интерфейс |
| --- | --- | --- | --- |
| Debian APT | `apt-cache policy`, `apt-get --simulate install` | install/remove/upgrade, без скрытого update | `apt(8)`, `apt-get(8)`, `apt-cache(8)`, `apt.conf(5)` |
| dpkg | `dpkg-query --show`, `dpkg --audit`, `dpkg-deb --info` | локальный deb, configure, remove | `dpkg(1)`, `dpkg-query(1)`, `dpkg-deb(1)` |
| RPM | `rpm -q`, `rpm -qp`, `rpmkeys --checksig` | транзакция с dependency resolver дистрибутива | `rpm(8)`, `rpm-queryformat(7)`, `rpmkeys(8)`, `dnf(8)` |
| alien | inspect исходного пакета и план конвертации | конвертация в изолированном build каталоге, без установки | `alien(1)` или `alien(1p)` конкретного дистрибутива |
| snapd | `snap list`, `snap info`, `snap changes`, `snap tasks ID` | install/refresh/remove/revert по change ID | `snap(8)`, `snap help COMMAND`, snapd API |

Для scripts APT рекомендует `apt-get` / `apt-cache`, а не интерактивный
интерфейс `apt`. Симуляция `apt-get -s` не держит lock и может устареть до
применения; её результат не является гарантией будущей транзакции.
[apt(8)](https://manpages.debian.org/bookworm/apt/apt.8.en.html),
[apt-get(8)](https://manpages.debian.org/testing/apt/apt-get.8.en.html).

`dpkg` имеет промежуточные состояния: unpacked, half-configured,
triggers-pending и другие. Просто увидеть имя/версию недостаточно.
[dpkg(1)](https://manpages.debian.org/bookworm/dpkg/dpkg.1.en.html).

RPM различает query/verify и install/upgrade/erase. Пакет содержит scripts;
даже проверку installed files следует проектировать с явной политикой
verify scripts, а не считать любой `rpm -V` безусловно пассивным.
План RPM backend не подменяет DNF/Zypper решением зависимостей.
[rpm(8)](https://rpm.org/docs/6.0.x/man/rpm.8).

`alien` — конвертер, а не универсальный dependency resolver. Конверсия не
доказывает ABI-совместимость; системные init/libc пакеты запрещены нашим
будущим API. Не использовать `--install`, не переносить maintainer scripts
автоматически и не запускать сборку неизвестного пакета с root-правами.
[alien(1p)](https://manpages.debian.org/bookworm/alien/alien.1p.en.html).

Snap имеет собственный daemon и автоматические refresh. Установка, refresh,
hold/unhold и изменение channel — разные операции. Прерывание клиента
не считается доказательством остановки daemon change: backend должен
проверить change/tasks и при явной отмене использовать API abort.
Автоматические refresh не отключаются как побочный эффект.
[snap updates](https://snapcraft.io/docs/how-to-guides/manage-snaps/manage-updates/),
локальные `snap(8)` и `snap help abort`.

## Правила будущего API

1. Сначала capability/version probe и точный argv, locale, backend, repository,
   architecture и pinned version. Никаких glob/regex/начального `-` или
   управляющих суффиксов `+`/`-` в поле точного имени пакета.
2. Plan показывает install/remove/downgrade, essential/protected packages,
   размеры, зависимости, repository и проверку подписи. Нельзя молча добавлять
   `--force-*`, `--allow-unauthenticated`, `--nogpgcheck` или `--nodeps`.
3. Реальный lock получает package manager. yesh не удаляет lock-файлы,
   не убивает владельца и не пытается заменить fcntl lock через `flock`.
   Ожидание lock конечное, отдельное от времени транзакции.
4. Noninteractive policy задаётся явно: `-y` не решает все вопросы debconf,
   conffiles и hooks. `DEBIAN_FRONTEND=noninteractive` само по себе не
   гарантирует успешную установку и не является глобальной настройкой.
5. Установка выполняет scripts/triggers с правами package manager и может
   перезапустить службы. Это изменение системы, не «безопасный файл».
6. Timeout установки не означает rollback. После отмены/сигнала результат
   должен быть interrupted/needs-audit; повтор и `dpkg --configure -a`,
   `apt-get -f install`, autoremove/purge требуют отдельного плана.
7. Текущий пяти минутный timeout внешней команды не годится как универсальный
   бюджет package transaction. До отдельного backend длинные установки
   лучше выполнять штатным менеджером, не менять runtime default ради APT.
8. В cron сначала только queries/audit. Автоматические upgrades — отдельная
   политика с maintenance window, lock conflict с unattended-upgrades,
   обработкой reboot-required и отчётом; не `curl | sh`, `yes | apt` или
   `apt ... | head`. Для снап daemon нужен проверенный доступ к сокету.

## Проверки без изменения хоста

Обязательны fixtures с argv/stdout/stderr/status для каждого backend: отсутствующий
пакет, broken database, missing executable, lock busy, недоступный snapd,
подпись/architecture mismatch, plan drift и timeout во время scripts.
Реальные install/remove/convert tests — только disposable VM/container с
отдельной package database и repository; namespace сам по себе не даёт
полной изоляции maintainer scripts. В этой работе пакеты не устанавливаются.
