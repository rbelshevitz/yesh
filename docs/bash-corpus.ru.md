# Корпус Bash-сценариев для адаптации

Цель корпуса — проверить yesh на реальных административных задачах, а не
переносить чужой код целиком. Для каждого случая фиксируем исходную задачу,
пишем независимую адаптацию на yesh и сравниваем план выполнения через
`--plan`.

## Источники с явной лицензией

| Источник | Лицензия | Подходящие задачи |
| --- | --- | --- |
| [dataCore/bash-scripts-collection](https://github.com/dataCore/bash-scripts-collection) | MIT (указана в репозитории) | SSH-hardening, Docker backup/restore, mountpoint, обновления Debian |
| [wesleyscholl/bash-scripts](https://github.com/wesleyscholl/bash-scripts) | MIT ([LICENSE](https://github.com/wesleyscholl/bash-scripts/blob/main/LICENSE)) | диски, backup/rsync, ротация логов, SSL и сервисы |
| [pedroetb/rsync-incremental-backup](https://github.com/pedroetb/rsync-incremental-backup) | MIT (указана в README и репозитории) | инкрементальный rsync, retention, include/exclude, SSH |
| [errnair/sysadmin-shell-scripts](https://github.com/errnair/sysadmin-shell-scripts) | MIT (указана в README и `LICENSE`) | system stats, backup конфигурации, аудит пользователей, syslog/JSON/CSV |
| [magnetikonline/rsync-backup-rotation](https://github.com/magnetikonline/rsync-backup-rotation) | MIT (указана на странице репозитория) | ротация rsync-каталогов и `--link-dest` |

Зафиксированные точки на 29 октября 2026:

- `pedroetb/rsync-incremental-backup` — `9909d78ac29bd128e90a3c39aed94643c6bf5fe7`, файл `rsync-incremental-backup-local`.
- `dataCore/bash-scripts-collection` — `d08fb8074ab71d562360ac7222c72d3ebae64643`, файлы `backup-docker-all.sh` и проверки `mountpoint`.
- `wesleyscholl/bash-scripts` — `468a774223c41e3816ea30f059de9b9321b81190`, файлы мониторинга диска и резервного копирования.
- `errnair/sysadmin-shell-scripts` — `621c2129e65a92103a4bf2e0905f72746a11e1f2`, `server_management/system_stats.sh` и `utilities/dirbackup.sh`.

Лицензия относится к исходному коду проекта. В yesh используем описание
поведения и пишем независимую реализацию; код не копируем. При буквальном
импорте нужно сохранить copyright/license notice и указать происхождение.

Репозитории без явной лицензии не используем как источники кода: публикация
на GitHub сама по себе не означает разрешение на копирование ([разъяснение GitHub](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/licensing-a-repository)).

## Правила отбора

1. Задача должна относиться к backup, systemd/journal, storage, SSH/SCP или
   сетевой проверке.
2. Входы и побочные эффекты должны быть наблюдаемы для сравнения `argv`, порядка
   действий и обработки ошибок.
3. Для каждой адаптации нужны положительный и отрицательный тесты.
4. Новая встроенная возможность появляется только если проблема повторяется
   минимум в двух независимых сценариях.

Пока это каталог кандидатов, а не импортированный код. Следующий этап —
выбрать 20–30 сценариев, зафиксировать URL и commit исходника, затем добавить
адаптации в `examples/` и тесты.
