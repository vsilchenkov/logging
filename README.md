# Логгер для GO на основе slog

Удобная обёртка над стандартным `log/slog` с человекочитаемым цветным выводом в
консоль, структурированным JSON-логом в файл и быстрым подключением приложения к
[Sentry](https://sentry.io/).

## Возможности

- Единый интерфейс `Logger` поверх `log/slog`.
- Цветной текстовый вывод в консоль ([tint](https://github.com/lmittmann/tint))
  или структурированный JSON в файл.
- Отправка событий в Sentry через `MultiHandler` (один вызов лога уходит сразу в
  несколько хендлеров) с отдельным порогом уровня для Sentry.
- Настраиваемый уровень логирования (от ошибок до debug).
- Удобные хелперы для создания атрибутов (`Str`, `Int`, `Err`, `Op`, …).
- Методы `With(...)` и `WithContext(...)` для предустановки атрибутов и привязки
  контекста.

## Установка

```bash
go get github.com/vsilchenkov/logging
```

Требуется Go 1.25+.

## Быстрый старт

```go
package main

import (
    "github.com/vsilchenkov/logging"
)

func main() {
    cfg := &logging.Config{
        BuildConfig: logging.BuildConfig{
            Version:     "1.0.0",
            ProjectName: "my-app",
            WorkingDir:  ".",
        },
        Level:        4,     // info
        OutputInFile: false, // вывод в консоль
    }

    sentryCfg := &logging.SentryConfig{
        Use: false, // Sentry выключен
    }

    log := logging.Initlogger(cfg, sentryCfg)

    log.Info("приложение запущено", log.Str("env", "dev"))
    log.Error("что-то пошло не так", log.Err(err))
}
```

После вызова `Initlogger` логгер сохраняется глобально и доступен через
`logging.GetLogger()` в любом месте приложения.

## Конфигурация

### `Config` — основной конфиг логгера

| Поле | Тип | Описание |
| --- | --- | --- |
| `Debug` | `bool` | Включить режим отладки. |
| `Level` | `int` | Уровень логирования (см. таблицу уровней ниже). |
| `OutputInFile` | `bool` | `false` — вывод в консоль (цветной текст), `true` — JSON в файл. |
| `Dir` | `string` | Каталог для файла логов (относительно `WorkingDir`). |

### `SentryConfig` — конфиг интеграции с Sentry

| Поле | Тип | Описание |
| --- | --- | --- |
| `Use` | `bool` | Включить отправку событий в Sentry. |
| `Dsn` | `string` | DSN проекта Sentry. |
| `Environment` | `string` | Окружение (`prod`, `staging`, …). |
| `Level` | `int` | Минимальный уровень событий, уходящих в Sentry. |
| `AttachStacktrace` | `bool` | Прикреплять стектрейс к событиям. |
| `TracesSampleRate` | `float64` | Доля трассируемых транзакций (0.0–1.0). |
| `EnableTracing` | `bool` | Включить трассировку производительности. |
| `Debug` | `bool` | Режим отладки Sentry SDK. |

### `BuildConfig` — общие данные сборки

Встраивается в `Config` и `SentryConfig`.

| Поле | Тип | Описание |
| --- | --- | --- |
| `Version` | `string` | Версия приложения (попадает в Sentry release). |
| `ProjectName` | `string` | Имя проекта (попадает в Sentry release). |
| `WorkingDir` | `string` | Рабочий каталог (база для пути к файлу логов). |

### Уровни логирования

`Level` задаётся числом. Если указано значение вне диапазона — используется
`info`.

| Значение | Уровень |
| --- | --- |
| `5` | Debug |
| `4` | Info (по умолч.) |
| `3` | Warn |
| `2` | Error |

## Вывод логов

### В консоль (`OutputInFile: false`)

Цветной человекочитаемый текст через `tint`, формат времени `15:04:05`, вывод в
`stderr`.

### В файл (`OutputInFile: true`)

JSON-формат, файл `app.log` в каталоге `WorkingDir/Dir`. Каталог создаётся
автоматически. Время форматируется как `2006-01-02 15:04:05`. Если файл открыть
не удалось — логгер откатывается на JSON-вывод в `stderr`.

## Интеграция с Sentry

При `SentryConfig.Use == true` основной хендлер и Sentry-хендлер объединяются в
`MultiHandler`: каждая запись уходит в оба назначения. Для Sentry действует
отдельный порог уровня (`SentryConfig.Level`); если он некорректен — берётся
уровень основного логгера.

Инициализацию самого Sentry SDK нужно выполнить отдельно, передав опции из
`SentryClientOptions`:

```go
import "github.com/getsentry/sentry-go"

if err := sentry.Init(logging.SentryClientOptions(sentryCfg)); err != nil {
    log.Fatalf("sentry init: %v", err)
}
defer sentry.Flush(2 * time.Second)

logger := logging.Initlogger(cfg, sentryCfg)
```

`SentryClientOptions` использует синхронный HTTP-транспорт с таймаутом 3 секунды
и формирует `Release` как `ProjectName@Version`.

## Интерфейс `Logger`

```go
type Logger interface {
    Debug(msg string, attrs ...slog.Attr)
    Info(msg string, attrs ...slog.Attr)
    Warn(msg string, attrs ...slog.Attr)
    Error(msg string, attrs ...slog.Attr)

    ErrorWithOp(msg string, err error, op string, attrs ...slog.Attr)

    With(attrs ...slog.Attr) Logger
    WithContext(ctx context.Context) Logger

    // Хелперы создания атрибутов
    Err(err error) slog.Attr
    Op(value string) slog.Attr
    Str(key, value string) slog.Attr
    Int(key string, value int) slog.Attr
    Float64(key string, value float64) slog.Attr
    Any(key string, value any) slog.Attr
}
```

### Методы логирования

```go
log.Debug("детали", log.Int("retry", 3))
log.Info("пользователь вошёл", log.Str("user_id", id))
log.Warn("медленный ответ", log.Float64("seconds", 4.2))
log.Error("ошибка БД", log.Err(err))
```

### `ErrorWithOp`

Удобный шорткат для логирования ошибки вместе с именем операции:

```go
log.ErrorWithOp("не удалось создать заказ", err, "OrderService.Create",
    log.Str("order_id", id))
```

Добавляет атрибуты `error` и `op` (если переданы непустыми) перед остальными
атрибутами.

### Хелперы атрибутов

| Метод | Назначение |
| --- | --- |
| `Err(err error)` | Атрибут `error`. |
| `Op(value string)` | Атрибут `op` (имя операции). |
| `Str(key, value string)` | Строковый атрибут. |
| `Int(key string, v int)` | Целочисленный атрибут. |
| `Float64(key, v float64)` | Атрибут с числом с плавающей точкой. |
| `Any(key string, v any)` | Произвольное значение. |

## With / WithContext

`With(...)` возвращает новый логгер с предустановленными атрибутами, которые
добавляются к каждой последующей записи. `WithContext(ctx)` возвращает новый
логгер, привязанный к контексту — контекст передаётся в slog при записи, что
позволяет Sentry-хендлеру извлечь из него hub/scope.

```go
log := logging.GetLogger()

// постоянные атрибуты
reqLog := log.With(log.Str("request_id", id), log.Str("user", user))
reqLog.Info("обработка запроса")

// привязка контекста (для Sentry scope/hub) + чейнинг
log.WithContext(ctx).With(log.Op("CreateOrder")).Error("ошибка БД", log.Err(err))
```

Оба метода создают **новый** экземпляр логгера и не меняют исходный — это
поведение в стиле slog (родительский логгер остаётся без изменений), безопасное
для конкурентного использования.

> **Внимание про контекст.** `WithContext` хранит `ctx` внутри логгера. Это
> отступление от рекомендации Go «не хранить Context в структуре», допустимое
> для логгеров. Чтобы избежать утечки отменённого/просроченного контекста,
> вызывайте `WithContext` рядом с местом использования, а не складывайте
> полученный логгер в долгоживущее поле структуры.

## Доступ к логгеру

```go
logging.Initlogger(cfg, sentryCfg) // инициализация + сохранение глобально
log := logging.GetLogger()         // получить глобальный логгер где угодно
```

При необходимости можно обернуть собственный `*slog.Logger`:

```go
log := logging.NewLogger(mySlogLogger)
```
