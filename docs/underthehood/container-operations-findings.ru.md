# Карта боли контейнеризированных операционных сценариев

Дата обзора источников: 4 ноября 2026 года.

Эта заметка собирает реальные обсуждения и первичные документы для дальнего
исследования yesh. Она не описывает реализованный интерфейс и не доказывает,
что yesh уже работает внутри контейнеров.

## Главный вывод

Боль не сводится к отсутствию `bash`. Она появляется там, где минимальное
окружение требует одноразовой последовательности действий, а порядок,
частичный отказ и проверка результата обычно прячутся в shell-склейке.

Наиболее перспективное направление — отдельный одноразовый операционный образ:

```text
yesh + явно нужные внешние программы + том/сеть
→ предварительная проверка → действие → проверка результата → код выхода
```

yesh не должен восстанавливать оболочку, заменять отсутствующие `tar`/`cp`,
становиться средой выполнения контейнеров или управлять кластером.

Проверяемая гипотеза о роли yesh такова: он уменьшает не сложность самой базы
или контейнерного оркестратора, а сложность связующего одноразового сценария
вокруг внешней команды. Такой сценарий проверяет входные условия, задаёт
порядок, ограничивает частичный отказ, показывает вызов и проверяет результат.
Если эти действия уже достаточно ясно выражены средствами мигратора или
системы развёртывания, yesh здесь не нужен.

## 1. Миграция базы как одноразовая операция

Здесь важно разделить две задачи. Мигратор меняет схему и знает, какие версии
уже применены. Операционный сценарий вокруг него проверяет окружение, ждёт
доступности базы, запускает мигратор в рамках выбранного запуска и передаёт
дальше только успешный результат.

Повторяющаяся проблема контейнерных развёртываний: миграция из
`initContainer` выполняется для каждого Pod, может повторяться при перезапуске
и конкурировать при масштабировании. Отдельный `Job` убирает часть гонок, но
добавляет порядок создания, ожидание завершения, повторный запуск и связь с
развёртыванием. Это уже задача системы развёртывания, а не то, что решает одна
локальная блокировка yesh.

- [Init Containers, Kubernetes](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/)
- [Jobs, Kubernetes](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [Миграция через Job или initContainer](https://www.reddit.com/r/kubernetes/comments/1v4d1m4/how_should_i_handle_database_migrations_in_a/)
- [Гонки и порядок миграций в нескольких репликах](https://www.reddit.com/r/kubernetes/comments/1qdcpf2/how_do_you_guys_run_database_migrations/)
- [Разовая задача миграции, Stack Overflow](https://stackoverflow.com/questions/37058812/how-best-to-run-one-off-migration-tasks-in-a-kubernetes-cluster)

Граница yesh: внешний мигратор и база владеют схемой, версиями и транзакциями.
Система развёртывания или сам мигратор владеют решением, какой узел запускает
миграцию. Локальная `with lock` yesh защищает только два процесса, которые
видят один и тот же локальный путь; она не доказывает единственный запуск на
нескольких узлах.

## 2. Права томов и непривилегированный процесс

`runAsUser`, `runAsGroup`, `fsGroup`, `subPath`, CSI, NFS, `hostPath` и rootless
режимы могут давать разные результаты. Рекурсивный `chown` может быть медленным,
запрещённым или менять не тот путь.

- [Security context и fsGroup, Kubernetes](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)
- [subPath и неожиданный root-владелец](https://www.reddit.com/r/kubernetes/comments/1j1lq1e/unexpected-subpath-behavior-in-kubernetes-auto-created-directories-with-root-ownership-and-permission-issues/)
- [Права init-контейнера для chown, Stack Overflow](https://stackoverflow.com/questions/70717175/kubernetes-use-the-same-volumemount-in-initcontainer-and-container)
- [fsGroup не всегда решает права Filestore](https://www.reddit.com/r/kubernetes/comments/1q6921a/best-practices-for-runasgroup-fsgroup-to-avoid-permissiondenied-on-filestore-mounts/)
- [Политика изменения владельцев больших томов, Kubernetes](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#configure-volume-permission-and-ownership-change-policy-for-pods)

Граница yesh: проверять ожидаемое назначение, UID/GID, владельца и права; не
обещать универсальное исправление прав и не выполнять опасный `chown` молча.

## 3. Образы без оболочки

Distroless-образ может не содержать shell, менеджера пакетов и обычных утилит.
Диагностика тогда выносится в отдельный debug-образ или ephemeral-контейнер.

- [Distroless: состав и debug-образы](https://github.com/GoogleContainerTools/distroless)
- [Диагностика Pod без утилит, Kubernetes](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/)
- [Практика отладки distroless](https://www.reddit.com/r/docker/comments/1uz926r/if_i_go_distroless_and_cant_exec_in_how_do_you/)
- [Ограничения exec в минимальном контейнере](https://www.reddit.com/r/kubernetes/comments/15tpohb/tips_for_troubleshooting_minimized_containers/)

Граница yesh: исследовать конечный отчёт из отдельного операционного или
диагностического образа. Не делать yesh универсальным shell внутри рабочего
образа.

## 4. PID 1 и сигналы

Shell-форма `ENTRYPOINT` может оставить приложение дочерним процессом shell;
тогда сигналы и завершение контейнера ведут себя неожиданно.

- [Shell- и exec-форма Dockerfile](https://docs.docker.com/reference/dockerfile/)
- [Проверка JSON-аргументов и PID 1, Docker](https://docs.docker.com/reference/build-checks/json-args-recommended/)
- [PID 1, `--init` и сигналы](https://docs.docker.com/reference/cli/docker/container/run)
- [Практический разбор `exec` и SIGTERM](https://www.reddit.com/r/docker/comments/1vdt7id/why_docker_stop_takes_ten_seconds_on_a_container/)

Теперь это отдельный вопрос приёмки одноразового контейнерного сценария.
Полезная гарантия узкая: если yesh работает как PID 1, он должен отменить и
очистить запущенную им группу процессов, дождаться детей и сообщить о
прерывании без публикации успеха. Это не превращает yesh в `tini`, супервизор
или менеджер процессов.

## 5. Секреты, конфигурация и сертификаты

Секрет, смонтированный как файл, может обновиться, но приложение обязано
перечитать его. Переменная окружения обычно требует перезапуска. Дальше
появляются reload, `SIGHUP`, перезапуск Pod и отдельные reloader-инструменты.

- [Secret как файл или переменная, Kubernetes](https://kubernetes.io/docs/concepts/configuration/secret/)
- [Обновление Secret не меняет состояние приложения автоматически](https://stackoverflow.com/questions/62913187/if-i-update-a-secret-does-it-automatically-gets-updated-to-all-resources-in-which-it-is-volume-mounted)
- [fsnotify, sidecar и перезапуск при смене TLS](https://www.reddit.com/r/kubernetes/comments/1l1n0w/whats-the-best-approach-for-reloading-tls-certs-in-kubernetes-prod-fsnotify-on-parent-dir-vs-sidecar-based-reloads/)
- [Перезапуск при изменении Secret или ConfigMap](https://www.reddit.com/r/kubernetes/comments/1hco0ge/reloadrestart-pod-on-file-changes/)

Граница yesh: конечная последовательность «проверить → вызвать внешний reload
или restart → проверить состояние». Постоянный наблюдатель, reloader и
контроллер остаются внешними системами.

## 6. Обновление стека и очистка узла

На одном контейнерном узле часто остаётся короткий cron-скрипт: получить новые
образы, пересоздать изменившиеся контейнеры, проверить результат и удалить
ненужное. Боль появляется в порядке действий и границе безопасности: старый
контейнер может продолжить работу, `prune` может освободить неожиданно много
места, а откат не подготовлен.

- [Очистка неиспользуемых объектов, Docker](https://docs.docker.com/engine/manage-resources/pruning)
- [Скрипт обновления Compose-стека](https://www.reddit.com/r/docker/comments/1k7gkny/using_docker_pull_but_with_a_compose_stack/)
- [Обновление оставило старые контейнеры работающими](https://www.reddit.com/r/selfhosted/comments/1vvr1u8/i_wrote_an_updater_that_printed_success_and_left/)
- [Неожиданный объём очистки Docker](https://www.reddit.com/r/selfhosted/comments/1fv7lih/reminder_to_prune_your_docker_hosts_once_in_a/)
- [Остановка контейнеров, резервная копия, обновление и очистка одним скриптом](https://www.reddit.com/r/selfhosted/comments/1fv7lih/reminder_to_prune_your_docker_hosts_once_in_a/)

Граница yesh: внешний `docker` или `podman` остаётся владельцем образов и
контейнеров. yesh может показать план обновления, проверить место и состояние,
дождаться проверки готовности и отложить очистку до успешного результата; он не
разбирает Compose как собственный язык и не обновляет весь узел автоматически.

## Приоритет исследования

1. Alpine-образ без `bash`, статический yesh, внешняя `sqlite3`, временный том и
   одна миграция;
2. тот же порядок на PostgreSQL в одноразовом `Job`;
3. проверка тома для непривилегированного процесса;
4. обновление одного внешнего Compose-стека с проверкой и отложенной очисткой;
5. конечная диагностика shell-less образа.

Для каждого сценария нужны положительный случай, частичный отказ, повторный
запуск, отрицательный случай без изменения данных и `--plan`. Пропуск из-за
отсутствия Docker, Podman, Kubernetes или нужного внешнего инструмента не
считать доказательством.
