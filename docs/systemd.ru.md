# systemd и journalctl: план упрощайзера

Цель — не заменить `systemctl` и `journalctl`, а убрать повторяющийся
административный шум и сделать опасные операции заметными при чтении.

## Целевой синтаксис

```yesh
service api

service.status api
service.restart api
service.wait api running within 30s
journal api since 15m lines 100
```

Сначала каждая операция должна раскладываться в обычные команды:

```text
systemctl is-active api
systemctl restart api
systemctl is-active --quiet api
journalctl -u api --since 15m -n 100 --no-pager
```

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

## Открытый вопрос

Нужно решить, будут ли `service.*` и `journal` встроенными типизированными
операциями или только безопасными макросами над `systemctl`/`journalctl`.
Для MVP предпочтительна внутренняя команда с тем же прозрачным планом argv.
