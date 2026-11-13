# Запуск сервиса

Сборка зависимостей и HTTP-сервер находятся в этом каталоге. Общий порядок запуска и адреса страниц описаны в [корневом README](../../README.md).

## Локально

Из корня проекта:

```powershell
$env:STORAGE = 'memory'
go run ./cmd/service
```

GraphQL доступен на `http://localhost:8081/graphql`, подписки — на `ws://localhost:8081/graphql`. Apollo Sandbox открывается на `http://localhost:8081/sandbox`; для загрузки интерфейса нужен интернет. Мутации различают автора по заголовку `X-User-ID`.

PostgreSQL из Compose, приложение с локальной машины:

```powershell
docker compose up -d --wait postgres
go run ./cmd/service
```

`STORAGE` можно не задавать: по умолчанию используется PostgreSQL. Сервис применяет миграции под advisory lock и открывает пул до приёма запросов. Ошибка миграции или соединения завершает процесс. Отдельно миграции запускаются так:

```powershell
go run ./cmd/migrate
```

Только контейнер приложения, без PostgreSQL и страницы проверки:

```powershell
docker compose run --build --rm --no-deps --service-ports -e STORAGE=memory app
```

Команда занимает порт `8081`. При обычном `docker compose up` контейнер PostgreSQL запускается как зависимость, даже если у приложения задано `STORAGE=memory`.

## Настройки

Файл настроек — [config.yaml](../../internal/config/config.yaml). Другой путь задаётся переменной `CONFIG_FILE`. Непустые переменные окружения заменяют поля YAML.

| Переменная | Назначение |
| --- | --- |
| `CONFIG_FILE` | Путь к YAML. По умолчанию `internal/config/config.yaml`. |
| `STORAGE` | `memory` или `postgres`. |
| `SANDBOX_ENABLED` | `true` или `false`. Заменяет `sandbox.enabled`. |
| `DATABASE_URL` | Строка подключения вместо полей PostgreSQL из YAML. |
| `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE` | Отдельные параметры PostgreSQL. При заданном `DATABASE_URL` pgx берёт их для параметров, которых нет в строке. |
| `HTTP_ADDR` | Адрес сервера, например `127.0.0.1:8081`. |
| `SHUTDOWN_TIMEOUT` | Положительная Go-duration для завершения HTTP-запросов. |

В YAML лежат демонстрационные учётные данные. В Compose хост базы для приложения — `postgres`: внутри контейнера `localhost` указывает на сам контейнер. Sandbox включён в пример конфигурации. Его отключают полем `sandbox.enabled: false` или переменной `SANDBOX_ENABLED=false`.

## Остановка

`SIGINT` и `SIGTERM` начинают остановку. GraphQL закрывает WebSocket-соединения, HTTP перестаёт принимать новые запросы и ждёт текущие до `SHUTDOWN_TIMEOUT`. Затем закрываются хаб подписок и пул базы. Если таймаут истёк, оставшиеся HTTP-соединения закрываются принудительно, и процесс завершается с ошибкой.
