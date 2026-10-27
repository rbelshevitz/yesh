# Упрощённый rsnapshot через yesh

Статус: предметный интерфейс ниже пока проектируется. Уже исполняемый
пример обычных команд — [rsnapshot-local.yesh](../examples/rsnapshot-local.yesh).
Реальные Go-тесты двух ротаций, восстановления версий и повторного
использования hard links описаны в [Р.П.П.](practical-examples.ru.md).

rsnapshot уже умеет сложную и полезную часть: incremental snapshots на базе
rsync и hard links, включая backup удалённых машин через SSH. yesh не должен
переписывать этот engine. Его задача — заменить трудно читаемый конфиг на
проверяемый сценарий.

## Целевой синтаксис

```yesh
snapshot root /mnt/backup/rsnapshot
require mounted /mnt/backup

keep daily 7
keep weekly 4
keep monthly 12

backup local /etc to local/etc
backup local /home to local/home
backup remote web-1 /etc to web-1/etc
backup remote web-1 /srv/app to web-1/app exclude [cache, tmp]
```

Запуск уровня остаётся явным:

```text
yesh backup.yesh --plan daily
yesh backup.yesh daily
```

`daily`, `weekly` и `monthly` — имена уровней retention, а не магическое
расписание. Время запуска задаётся cron/systemd timer или другим внешним
планировщиком.

## Что делает yesh

Перед запуском yesh проверяет:

1. snapshot root существует и является ожидаемой директорией;
2. backup filesystem действительно смонтирован;
3. mountpoint не является обычной директорией на root filesystem;
4. SSH host key и authentication работают без отключения проверки;
5. source paths доступны;
6. retention levels упорядочены и не дублируются;
7. `rsnapshot configtest` проходит;
8. в `--plan` видны rotation, rsync source/destination и опасные опции.

Для внешнего диска обязательно генерируется `no_create_root=1`: rsnapshot FAQ
предупреждает, что без этого при незамонтированном диске backup может уйти в
обычную директорию `/mnt` и заполнить локальный диск.

## Опасные операции

`backup remote` может удалять старые snapshots по retention policy, но не должен
удалять источник. `--delete` rsync не включается автоматически: если он нужен,
он указывается явно и показывается в плане.

```yesh
backup remote web-1 /srv/app to web-1/app delete extra
```

Без `--plan` destructive rotation требует доступного lock и проверки, что два
экземпляра не запускаются одновременно.

## Restore и inspect

Полезный frontend должен уметь не только создавать snapshots:

```text
yesh backup.yesh list daily
yesh backup.yesh diff daily web-1/app
yesh backup.yesh restore daily web-1/app ./restore
```

Restore по умолчанию идёт в новый target directory. Перезапись production-пути
требует отдельного явного флага.

## Почему не YAML

Конфигурация backup — это исполняемый операционный сценарий: проверки mount,
SSH, `--plan`, ошибки, restore и внешние команды. Поэтому yesh сохраняет
обычные списки, циклы и команды вместо отдельного YAML-DSL.

## Основа

rsnapshot использует rsync и hard links; retention задаётся уровнями `retain`,
а SSH применяется для удалённых источников. yesh скрывает только ритуальную
конфигурацию и добавляет safety checks, не меняя проверенный backup engine.

## Anti-monkey: защита от типовых ошибок

Практическая ценность frontend — не в том, чтобы спрятать параметры, а в том,
чтобы опасное действие требовало явного и проверяемого решения.

До запуска yesh проверяет:

- отсутствующий или не тот mountpoint — hard error;
- snapshot root на локальном root filesystem — hard error, если требовался
  mounted backup volume;
- неизвестный host или source path — hard error;
- отключённую SSH host-key verification — hard error;
- незаполненный retention или дублирующий уровень — hard error;
- второй экземпляр backup — lockfile;
- свободное место по заданной политике;
- отдельное отображение `--delete`, rotation и overwrite в плане.

yesh не должен создавать отсутствующий mountpoint и продолжать работу,
подменять remote source на local path, включать `StrictHostKeyChecking=no`,
считать успешным backup только по exit code rsnapshot или восстанавливать поверх
production-каталога без явного флага.

После backup проверяются ожидаемый snapshot, source roots, timestamp/manifest,
частичные ошибки по host и чтение небольшого контрольного файла.

Для restore целевой каталог по умолчанию должен быть новым или пустым:

```text
yesh backup.yesh verify daily web-1/app ./restore-check
```

Критерий anti-monkey:

> Обычная ошибка оператора должна остановить выполнение до разрушительного
> действия и объяснить, что именно нужно исправить.
