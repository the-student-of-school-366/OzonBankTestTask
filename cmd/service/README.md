# Запуск сервиса

Из корня проекта в PowerShell:

```powershell
# Хранилище в памяти.
$env:STORAGE='memory'
go run ./cmd/service
```

Apollo Sandbox доступен на `http://localhost:8081/sandbox` (для загрузки
интерфейса нужен интернет). GraphQL HTTP и WebSocket доступны на
`http://localhost:8081/graphql` и `ws://localhost:8081/graphql`. Подробности операций и подписок — в
[README API](../../internal/api/README.md). Для демонстрации мутаций передаётся
заголовок `X-User-ID`; он не является аутентификацией.

Запуск приложения и PostgreSQL через Compose:

```powershell
docker compose up --build -d --wait
```

Сервис доступен на `http://localhost:8081/graphql`. Compose ждёт готовности
PostgreSQL перед запуском приложения; приложение применяет миграции при старте.
Учётные данные и имя базы можно задать через `POSTGRES_USER`, `POSTGRES_PASSWORD`
и `POSTGRES_DB`. PostgreSQL применяет их при первой инициализации тома: если том
уже существует, новые значения нужно также изменить в самой базе. `docker compose down`
останавливает контейнеры и сохраняет том с данными.

Для запуска только приложения в режиме памяти, без контейнера PostgreSQL:

```powershell
docker compose run --build --rm --no-deps --service-ports -e STORAGE=memory app
```

Эта команда использует порт 8081; сначала остановите другое приложение на этом порту.
При обычном `docker compose up` PostgreSQL запускается как зависимость даже при
`STORAGE=memory`.

Итоговый образ содержит статически собранный сервис и минимальное окружение с
сертификатами CA; компилятор Go остаётся на этапе сборки. Процесс работает от
непривилегированного пользователя. Настройки читаются из YAML; Compose заменяет
хост базы именем `postgres`. Адрес `localhost` внутри контейнера указывает на
сам контейнер.

Для локального запуска сервиса с PostgreSQL из Compose:

```powershell
docker compose up -d --wait postgres
$env:STORAGE='postgres' # можно не задавать: PostgreSQL используется по умолчанию
go run ./cmd/service
```

При старте PostgreSQL-режима сервис применяет миграции под advisory lock и
открывает пул до начала приёма запросов. Ошибка миграции или соединения завершает
процесс с ошибкой. Для отдельного применения миграций доступна `go run ./cmd/migrate`.

Основные настройки находятся в [config.yaml](../../internal/config/config.yaml).
Локально путь к другому файлу задаётся через `CONFIG_FILE`; в контейнере такой
файл должен быть доступен по указанному пути. Непустые переменные окружения
переопределяют YAML:

| Переменная | Назначение |
| --- | --- |
| `CONFIG_FILE` | Путь к YAML; по умолчанию `internal/config/config.yaml`. |
| `STORAGE` | `memory` или `postgres`. |
| `SANDBOX_ENABLED` | `true` или `false`; переопределяет `sandbox.enabled` в YAML. |
| `DATABASE_URL` | Используется вместо параметров PostgreSQL из YAML. |
| `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE` | Отдельные параметры PostgreSQL. При заданном `DATABASE_URL` pgx может использовать их для отсутствующих в строке параметров. |
| `HTTP_ADDR` | Адрес TCP-сервера, например `127.0.0.1:8081`. |
| `SHUTDOWN_TIMEOUT` | Положительная Go-duration для завершения HTTP-запросов. |

В YAML находятся только демонстрационные учётные данные; настоящие секреты
следует хранить вне репозитория. В Docker Compose PostgreSQL по умолчанию имеет
те же учётные данные, а его хост для приложения — `postgres`.
Sandbox включён в пример конфигурации для разработки; отключите его значением
`sandbox.enabled: false` или `SANDBOX_ENABLED=false`, если интерфейс не нужен.

SIGINT/SIGTERM запускают остановку: GraphQL закрывает WebSocket-соединения, HTTP
перестаёт принимать новые запросы и ждёт текущие до таймаута, затем закрываются
хаб подписок и пул базы. При превышении таймаута оставшиеся HTTP-соединения
закрываются принудительно, а процесс сообщает об ошибке.
