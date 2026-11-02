# yesh и BusyBox

## Установленное окружение

Пользователь установил `/usr/bin/busybox`: версия 1.36.1, Ubuntu-сборка
`1:1.36.1-6ubuntu3.1`. Наличие бинарного файла и списка applet-команд проверено;
это пока не испытание rescue-профиля или совместимости всех параметров.

Текущая проверка безопасности запрещает `busybox applet` в конвейере как
обёртку запуска, способную вызвать `busybox sh`. Для будущих проверок нужны
отдельный PATH с applet-ссылками и/или явная проверка допустимой команды
multicall-бинаря. Нельзя просто убрать защиту и объявить совместимость.

## Граница ответственности

BusyBox — один multicall-бинарь с набором applet-команд: `mount`, `ps`,
`blkid`, `grep`, `sed`, `awk`, `ip`, `switch_root` и другими. Команду можно
вызывать как `/bin/busybox applet`, а обычно используются symlink’и на один
бинарь. [BusyBox applets](https://busybox.net/downloads/BusyBox.html)

yesh не должен становиться вторым BusyBox и не должен переписывать `mount`,
`sed`, `awk` или `ps`. Его роль — сделать последовательность действий
читаемой и безопасной:

```yesh
root = device by uuid "..."
mount root at /sysroot readonly
require mounted /sysroot
run switch_root /sysroot /sbin/init
```

На ранней системе эти операции могут раскладываться на BusyBox applet’ы.
На полноценной системе — на util-linux, systemd и обычные Unix-команды.

## Минимальный набор для initramfs

### Обязательный baseline

- `sh`/`ash` — только как аварийный fallback, не как backend yesh;
- `mount`, `umount`, `mountpoint`, `switch_root`;
- `blkid`, `findfs`, `ls`, `cat`, `readlink`, `stat`;
- `mkdir`, `mknod`, `chmod`, `chown`, `ln`, `cp`, `mv`, `rm`;
- `grep`, `sed`, `cut`, `tr`, `sort`, `uniq`, `head`, `tail`, `wc`;
- `ps`, `pgrep`, `pidof`, `kill`, `killall`, `sleep`, `timeout`;
- `dmesg`, `logger` или `logread`;
- `modprobe`, `lsmod`, `depmod`, если драйверы модульные;
- `ip`, `udhcpc`, если root должен находиться по сети.

Конкретный список зависит от initramfs: ненужные applet’ы не включаются
только ради полноты. Для root на LUKS/LVM/RAID потребуются соответствующие
внешние инструменты и kernel support; yesh не должен притворяться, что одного
`mount` достаточно.

## Что должно быть встроено в yesh

Минимум встроенного runtime:

- запуск argv без скрытого shell;
- stdin/stdout/stderr, пайпы и редиректы;
- `if`, конечный `for`, списки и строки;
- timeout и отмена всей process group;
- `--plan`;
- `cd`, рабочий каталог и явное окружение;
- read-only preflight для устройства, mountpoint и существования файла;
- понятная ошибка с командой, стадией pipeline и причиной.

`echo`, `true` и `false` допустимы как маленькие встроенные команды. Остальные
базовые команды лучше оставить внешними: это сохраняет совместимость с BusyBox
и не раздувает runtime.

## Режим rescue/initramfs

Отдельный будущий режим:

```text
yesh --rescue /init.yesh
```

Он должен:

- проверять, что runtime запущен как ожидаемый PID/процесс;
- не выполнять `sudo`, systemd API и недоступные desktop-интеграции;
- показывать доступные applet’ы и отсутствующие зависимости;
- использовать только конечные retry с backoff;
- перед `switch_root` проверять source, filesystem, mountpoint и наличие init;
- при ошибке оставлять диагностическую shell только по явному запросу.

Go-бинарь для такого режима должен собираться без CGO и с `-trimpath`; размер
будет больше BusyBox, поэтому yesh не заменяет BusyBox в минимальном образе.
Практическая связка: BusyBox предоставляет инструменты, yesh — читаемый
оркестратор и safety checks.

## Не включать в baseline

- встроенные копии `sed`, `awk`, `ps`, `mount` и `ip`;
- автоматическое обнаружение и загрузку всех модулей;
- собственную реализацию init, udev, cryptsetup, LVM или mdadm;
- бесконечный rescue loop;
- молчаливый fallback на другую команду при отсутствии applet’а.
