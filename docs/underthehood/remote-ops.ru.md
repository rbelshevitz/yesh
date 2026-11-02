# Remote operations: SSH, SCP, rsync и mount

Это главный практический сценарий yesh.

## Какую проблему решает yesh

yesh должен убрать ручную сборку последовательности SSH, SCP, rsync и проверок,
не скрывая адрес, порядок и аргументы. OpenSSH и внешние команды остаются
ответственными за ключи, доверие к узлу, сеть и права; yesh не превращает
подключение к другому узлу в скрытую оболочку.

## Проверить узел — не значит найти узел

В резервном копировании обычно уже известны имя или адрес источника. Тогда
сценарий может последовательно проверить:

```text
имя → DNS → маршрут → SSH и ключ узла → каталог → rsync --dry-run → rsync
```

В yesh это сегодня делается видимыми внешними командами: `getent ahosts`,
`ip route get`, `ssh` с `BatchMode=yes` и `StrictHostKeyChecking=yes`, затем
`rsync --dry-run`. Их порядок, аргументы и ошибки попадают в обычный сценарий и
показываются в `--plan`.

Это проверка конкретного источника, а не поиск машин в подсети. Успешный ответ
DNS или маршрут не доказывает, что узел доступен; доступный SSH-узел не доказывает,
что это нужная машина; найденный адрес не заменяет проверку ключа узла и прав.

Поэтому yesh пока не сканирует сеть и не делает вид, что обнаружение хостов —
готовый источник истины. Список узлов приходит из окружения, файла или внешнего
инвентаря, а yesh проверяет каждый явно заданный источник перед резервным
копированием. Для практического примера используйте
[`rsync-network-preflight.yesh`](../../examples/rsync-network-preflight.yesh) и
[`rsync-networked-incremental-rwe.yesh`](../../examples/rsync-networked-incremental-rwe.yesh).

## Базовый стиль

```yesh
hosts = [web-1, web-2]

for host in $hosts:
    ssh host systemctl restart api
    scp build/api host:/srv/api/api
    rsync -az build/ host:/srv/api/
```

`ssh`, `scp`, `rsync` и `mount` остаются внешними Linux-командами. yesh не заменяет их
протоколы и не хранит ключи: OpenSSH agent, `~/.ssh/config`, `known_hosts`,
ProxyJump и host-key policy остаются внешней конфигурацией.

## Typed remote path

Строка `host:/path` опасна для простого текстового interpolator: в ней могут
быть IPv6, пробелы и переменные. Поэтому целевой синтаксис — отдельное значение
remote path:

```yesh
target = remote host /srv/api
rsync -az build/ target
```

Runtime сериализует его в корректный аргумент для конкретной утилиты:

```text
web-1:/srv/api
[2001:db8::10]:/srv/api
```

Это не пользовательская объектная модель, а проверяемое значение аргумента.
Нельзя склеивать remote path через строковую конкатенацию.

## SSH-команды

OpenSSH принимает назначение и затем команду на другом узле; дополнительные
аргументы соединяются пробелами и передаются удалённой стороне. Поэтому yesh
не должен передавать туда неэкранированный текст, полученный из переменной.

Целевой runtime для формы:

```yesh
ssh host systemctl restart api
```

строит remote command из отдельных аргументов с POSIX-safe quoting. Shell
операторы, подстановка команд и вложенная оболочка запрещены разбором.

Запрещено по умолчанию:

```yesh
ssh host bash -c text
ssh host "$(make_command)"
```

Для интерактивной команды нужен отдельный явный режим, например `ssh -t` или
будущий блок `interactive`; обычный admin-скрипт не должен зависать, ожидая
ввод от процесса на другом узле.

## SCP

`scp` используется для простого копирования. В новых OpenSSH он использует
SFTP-протокол по SSH по умолчанию; legacy protocol через `-O` не включается
автоматически.

```yesh
scp config.toml remote host /etc/myapp/config.toml
```

Если остаётся нативный синтаксис `host:/path`, yesh должен валидировать и
канонизировать remote path до передачи в `scp`. Не отключать host-key checking.

## Rsync

`rsync` остаётся внешним инструментом и получает отдельные аргументы:

```yesh
rsync -a --delete build/ target
```

`--delete` является опасной операцией и должен отображаться в `--plan` с
явным предупреждением. Различие `build/` и `build` должно сохраняться в AST:
trailing slash меняет семантику rsync и не может теряться при normalizing path.

Для remote-shell transport yesh не подменяет `-e ssh` и не передаёт секреты в
командной строке. SSH-аутентификация и host verification остаются у OpenSSH.

## Mount

Mount — операция с изменением состояния машины, поэтому в MVP это внешняя
команда с обязательным явным target:

```yesh
mount -t nfs server:/exports/app /mnt/app
```

Для SSHFS:

```yesh
sshfs host:/srv/data /mnt/data
umount /mnt/data
```

Позже можно добавить `ensure mount`, но не скрывать mount/unmount внутри `ssh`
или `rsync`. `--plan` должен показывать mountpoint, filesystem type и опции,
а timeout и cleanup должны учитывать, что mount может породить отдельный
процесс или зависнуть на сетевом filesystem.

## Ограничения первого remote MVP

- hosts — конечный список;
- запуск последовательно, без неявного fan-out;
- явная параллельность появится позже с лимитом concurrency;
- credentials только через OpenSSH agent/config и внешнее окружение;
- no `sshpass`, no `StrictHostKeyChecking=no`;
- каждый remote command имеет timeout и полный exit status;
- ошибка на одном host останавливает rollout, если явно не выбран режим
  collect-errors.
