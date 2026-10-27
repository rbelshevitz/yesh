# Man-страницы — часть интерфейса

У yesh есть собственная [yesh(1) на русском](../man/ru/yesh.1)
и [на английском](../man/yesh.1): параметры запуска, переменные,
pipeline, редиректы, сигналы, cron, ограничения защиты и коды выхода.
Английский справочник языка — [yesh-language(7)](../man/yesh-language.7).

```bash
man -l man/ru/yesh.1
MANPAGER=cat man -l man/ru/yesh.1
```

Страницы пока не устанавливаются в системные каталоги автоматически.
Будущая команда установки должна копировать исполняемый файл, английскую
страницу в `share/man/man1/yesh.1`, русскую — в `share/man/ru/man1/yesh.1`
внутри выбранного каталога установки, без изменения MANPATH и настроек man.

Для внешних backend локальный man имеет приоритет при проверке доступных
опций установленной версии; онлайн manual нужен для проектирования и ссылок.
Номер раздела задаётся явно: `crontab(1)` управляет таблицей,
`crontab(5)` описывает формат. Если man-пакет отсутствует, сообщаем об этом,
а не угадываем флаги по названию команды.

| Подсистема | Читать до реализации |
| --- | --- |
| Процессы и поток | `signal(7)`, `pipe(7)`, `setpgid(2)`, `kill(2)`, `waitpid(2)`, `open(2)` |
| Планировщик | `cron(8)`, `crontab(1)`, `crontab(5)`, `flock(1)`, `timeout(1)` |
| Debian | `apt(8)`, `apt-get(8)`, `apt-cache(8)`, `apt.conf(5)`, `dpkg(1)`, `dpkg-query(1)`, `dpkg-deb(1)` |
| RPM | `rpm(8)`, `rpmkeys(8)`, `rpm-queryformat(7)`, man resolver дистрибутива |
| Конверсия | `alien(1)` / `alien(1p)` |
| Snap | `snap(8)`, `snap help`, `snap help COMMAND` |
| Права | `chmod(1)`, `chown(1)`, `chgrp(1)`, `chmod(2)`, `chown(2)`, `acl(5)`, `umask(2)`, `path_resolution(7)`, `openat2(2)`, `capabilities(7)` |

В текущем WSL локально прочитаны `crontab(5)`, `flock(1)`, `timeout(1)`,
`apt(8)`, `dpkg-query(1)`, `snap(8)`; RPM и alien отсутствуют,
использованы официальные онлайн man, ссылки в [packages.ru.md](packages.ru.md).
Наличие client binary не доказывает доступность daemon или права на операцию.
