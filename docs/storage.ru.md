# Хранилища, блочные устройства и fstab: план

Работа с дисками — отдельный safety-контур. Нельзя сводить её к строковой
команде `mount`: ошибка устройства или mountpoint может повредить данные.

## Целевые операции

```yesh
disk = device by uuid "..."
inspect disk
mount disk at /mnt/backup readonly
require mounted /mnt/backup
unmount /mnt/backup
```

Диагностика должна использовать явные системные источники:

- `blkid` для UUID, PARTUUID, TYPE и LABEL;
- `lsblk` для дерева устройств, mountpoint и read-only состояния;
- `findmnt` для проверки фактической точки монтирования;
- `/etc/fstab` только через структурированное чтение и проверку записи;
- `stat`, `df` и `du` для размера, свободного места и заполнения.

## Анти-мартышка

- нельзя монтировать по `/dev/sdX` без подтверждённой идентичности устройства;
- UUID/PARTUUID должны однозначно разрешаться, иначе операция останавливается;
- mountpoint обязан быть каталогом и не должен быть неожиданно занят;
- перед backup проверяется, что путь действительно находится на нужной FS;
- `require mounted` падает, если mount отсутствует или смонтирован не тот источник;
- запись в `/etc/fstab` не делается автоматически в MVP;
- `umount` и `mount` получают timeout и видимую ошибку busy;
- операции над разделами, `mkfs`, `wipefs`, `dd`, `fdisk` и `parted` запрещены
  встроенным API MVP;
- `--plan` и dry-run не должны притворяться проверкой реального состояния.

## Последовательность безопасного backup

```text
resolve UUID → inspect → verify filesystem → verify mountpoint
→ verify free space → run backup → verify result → report source and target
```

Приоритет — read-only диагностика. Изменяющие операции добавляются только с
отдельным типом действия, preflight-проверками и тестом на неправильный диск.
