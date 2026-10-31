# CIDR в yesh

CIDR — встроенный тип для сетевой автоматизации, а не строковая операция.

В текущем runtime уже реализованы строгие значения `ip`, `cidr`, операции
`masked` и условия `contains`/`overlaps`. Калькулятор границ, размеров и
перебора ниже пока остаётся
следующей частью интерфейса и не должен выдаваться за реализованный API.
Правила основаны на [RFC 4632](https://www.rfc-editor.org/rfc/rfc4632),
[RFC 4291](https://www.rfc-editor.org/rfc/rfc4291) и канонической текстовой
форме IPv6 из [RFC 5952](https://www.rfc-editor.org/rfc/rfc5952).

## Строгий ввод

```yesh
network = cidr 192.0.2.17/24
ip = ip 192.0.2.42

if contains $network $ip:
    echo ip "belongs to" network
```

Правила:

- prefix length обязателен;
- для IPv4 допустим диапазон `/0`–`/32`;
- для IPv6 допустим диапазон `/0`–`/128`;
- маска должна быть непрерывной, без classful-угадывания;
- IPv4 и IPv6 не смешиваются молча;
- невалидный ввод останавливает скрипт до запуска команды;
- адрес и prefix — разные значения: host bits не исчезают при parse;
- canonical network получается отдельной операцией `masked`.

`/0` — валидный default route, но его нельзя автоматически разворачивать в
список адресов.

## Канонический вывод

```text
input:     2001:0DB8:0:0:0:0:0:1/32
canonical: 2001:db8::1/32
```

IPv6 принимается в любой легальной форме RFC 4291, но печатается в единой
канонической форме RFC 5952: lowercase hex, самая длинная последовательность
нулевых групп сжимается через `::`, а неоднозначные адреса с портом пишутся как
`[2001:db8::1]:443`.

## Калькулятор

Минимальный набор операций:

```yesh
network = cidr 10.20.30.17/24

canonical = masked $network

if overlaps $canonical $network:
    echo "prefixes overlap"

first_address = first $network
last_address = last $network

if contains $network $first_address:
    echo "first address is inside the prefix"

show network
show masked network       # 10.20.30.0/24
show contains network ip
show size network
show first network
show last network
show overlaps network other_network
```

Семантика `first` и `last` — первый и последний адрес диапазона. Это не
«первый usable host»: исключение network/broadcast является отдельной политикой
IPv4, а для IPv6 вообще нет универсального broadcast-правила. `/31` и `/32`
также нельзя обрабатывать старой логикой «вычесть два адреса».

## Итерация

Разворачивание диапазона всегда ограничено явно:

```yesh
for address in hosts network limit 256:
    ping address
```

Правила защиты:

- `for address in network` без `limit` — ошибка;
- лимит должен быть положительным и ограничен общим execution budget;
- превышение лимита обнаруживается до первого запуска команды;
- `--plan` показывает prefix и limit, но не печатает миллионы адресов;
- вычисления `contains`, `overlaps`, `masked`, `size` не материализуют список.

## Реализация

В Go используем стандартный `net/netip`: `Addr` и `Prefix` — immutable value
types с `ParseAddr`, `ParsePrefix`, `Contains`, `Overlaps`, `Masked`, `Next` и
`Prev`. После `ParsePrefix` yesh явно вызывает `Masked`, когда нужен именно
канонический сетевой prefix: Go сохраняет host bits во входном `Prefix`.

Никаких сторонних IP-библиотек и самодельного битового парсинга.

## Предлагаемая политика IPv6

IPv6 — не урезанный специальный случай, а полноценное значение того же типа
`ip`. Пользователю не нужны отдельные `ipv4` и `ipv6`, но yesh всегда знает
семейство адреса и не делает неявное преобразование.

### Зоны интерфейсов

Link-local адрес может иметь scope zone:

```yesh
router = ip fe80::1%eth0
```

Зона относится к адресу интерфейса, а не к CIDR prefix:

```yesh
link = cidr fe80::/64
```

Правила:

- zone разрешена у `ip`, но запрещена у `cidr`;
- `contains` не удаляет zone молча;
- сравнение адресов с разными zone требует явного `unzone` или даёт ошибку;
- `fe80::1%eth0` нельзя отправить на интерфейс без подтверждённого scope;
- в URI/host:port IPv6 всегда заключается в квадратные скобки.

Это предотвращает ситуацию, когда одинаково выглядящие link-local адреса
указывают на разные интерфейсы.

### Не путать network, broadcast и host range

В yesh нет универсального свойства `broadcast` для prefix. Вместо него есть:

```yesh
show first network
show last network
```

`first` и `last` означают границы математического диапазона. Политика
«usable hosts» не встраивается: у IPv4 и IPv6 разные operational conventions,
а `/31` и `/32` нельзя обрабатывать арифметикой старых сетевых калькуляторов.

### Адресная арифметика

```yesh
next = next address
previous = prev address
```

Переход за `ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff` или ниже нуля — ошибка,
не wraparound. Любая генерация адресов ограничена `limit` и общим execution
budget.

### Классификация

Реализованы безопасные предикаты без ручного сравнения строк:

```yesh
if is link-local $address:
    ...
if is multicast $address:
    ...
if is global-unicast $address:
    ...
if is loopback $address:
    ...
```

`is private`, `is loopback`, `is multicast` и `is global-unicast` должны
опираться на бинарное значение адреса и таблицы стандартной библиотеки, а не на
текстовый prefix-match.

### IPv4-mapped IPv6

`::ffff:192.0.2.1` по умолчанию остаётся IPv6-значением. Автоматическое
превращение его в IPv4 опасно для ACL и сравнений. Для этого нужна явная
операция `unmap`.

### Operational prefixes

Калькулятор не навязывает правило `/64` для любого IPv6. `/127` разрешён для
межмаршрутизаторных point-to-point links по [RFC 6164](https://www.rfc-editor.org/rfc/rfc6164),
и его нельзя превращать в «нулевое число usable hosts».

`fc00::/7` классифицируется как Unique Local Unicast по [RFC 4193](https://www.rfc-editor.org/rfc/rfc4193),
но это не означает, что любой адрес с таким prefix автоматически безопасен или
маршрутизируем только локально в конкретной инфраструктуре.

Предикаты `is private`, `is global`, `is reserved` и `is special` должны
опираться на версионируемые данные IANA, а не на короткий список в коде.

## Read-only отчёт интерфейсов

Для наблюдения интерфейсов и адресов yesh оставляет `ip` внешней командой:

```yesh
ip -br address show
```

`--plan` показывает этот вызов, а отказ доступа или отсутствие `ip` остаются
ошибкой. Встроенных `add`, `change`, `replace` и `flush` для адресов нет.

Таблица маршрутов также остаётся внешним read-only вызовом:

```yesh
ip route show
```

Она не смешивается с `ip route get`: первый вызов показывает наблюдаемую
таблицу, второй проверяет выбор маршрута для конкретного адреса.

Соседей и слушающие сокеты также можно читать без встроенного сетевого API:

```yesh
ip neigh show
ss -H -n -l -t -u | head -n 200
```

Отсутствие записи соседа не означает недоступность узла. `ss` остаётся
внешним отчётом: yesh ограничивает число строк, но не разбирает его колонки.
Сведения о владельцах и PID могут требовать прав; для них оператор явно
запускает весь скрипт через `sudo`. Скрытого повышения прав и автоматического
повтора с `sudo` нет.
Реестр special-purpose адресов прямо предупреждает, что принадлежность к нему
не гарантирует routability в конкретном контексте. [IANA IPv4 registry](https://www.iana.org/assignments/iana-ipv4-special-registry),
[IANA IPv6 registry](https://www.iana.org/assignments/iana-ipv6-special-registry)
