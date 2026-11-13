# Посты и комментарии на GraphQL

Сервис на Go предоставляет GraphQL API для постов, вложенных комментариев и
подписок на новые комментарии. Хранилище выбирается при запуске: PostgreSQL или
память процесса. Примеры ниже рассчитаны на PowerShell в корне проекта.

## Запуск через Docker Compose

Нужны установленный и запущенный Docker с Compose. Порты `8081` и `55432` на
локальном компьютере должны быть свободны.

```powershell
docker compose up --build -d --wait
docker compose ps
```

Compose собирает приложение и запускает PostgreSQL. Приложение применяет
миграции при старте. По умолчанию используется PostgreSQL, как задано в
[`internal/config/config.yaml`](internal/config/config.yaml); непустая переменная
`STORAGE` может переопределить это значение. HTTP GraphQL доступен на `http://localhost:8081/graphql`,
WebSocket — на `ws://localhost:8081/graphql`. Это API, а не страница с
интерактивным редактором запросов. Если первый запрос пришёл до завершения
миграций, дождитесь сообщения `service started` в журналах:

```powershell
docker compose logs -f app
```

Для выхода из просмотра журналов нажмите `Ctrl+C`: контейнеры продолжат работу.
Если порт занят или приложение не запустилось, проверьте `docker compose ps` и
журналы `docker compose logs app postgres`.

По умолчанию Compose создаёт базу `ozon_bank` с пользователем `ozon` и паролем
`ozon_dev` — эти же локальные значения приведены в YAML. Не используйте пример
пароля для настоящих секретов. Учётные данные и имя базы Compose можно задать
через `POSTGRES_USER`, `POSTGRES_PASSWORD` и `POSTGRES_DB` **до первой
инициализации тома**. У существующей базы изменение этой переменной само по себе
не меняет учётные данные: их нужно также изменить в PostgreSQL. Эти переменные
также передаются приложению. Том `postgres_data`
сохраняет данные между перезапусками.

На хосте Compose использует `8081` для приложения и `55432` для PostgreSQL;
их можно изменить переменными `APP_PORT` и `POSTGRES_PORT` соответственно.
Внутри сети Compose приложение подключается к собственной базе по адресу
`postgres:5432`, независимо от внешнего порта базы.

### Настройка приложения

В [`internal/config/config.yaml`](internal/config/config.yaml) задаются тип
хранилища, HTTP-хост и порт, таймаут остановки и параметры PostgreSQL: хост,
порт, пользователь, пароль, имя базы и SSL. Отредактируйте файл перед запуском;
для Docker-образа после изменения выполните `docker compose up --build -d --wait`.
Compose переопределяет адрес базы на `postgres:5432` внутри своей сети; при
локальном запуске Go приложение использует `localhost:55432` из YAML.
При изменении HTTP-порта или хоста проверьте сопоставление портов в Compose и
сохраните адрес, доступный извне контейнера, например `0.0.0.0`.

`CONFIG_FILE` задаёт путь к другому YAML-файлу. Без него используется
`internal/config/config.yaml` относительно рабочего каталога процесса. В Docker
файл по умолчанию скопирован в образ; для другого файла его нужно также
смонтировать в контейнер. Пустые переменные окружения игнорируются; непустые
имеют приоритет над YAML:

| Переменная | Что переопределяет |
| --- | --- |
| `STORAGE` | `postgres` или `memory` |
| `HTTP_ADDR` | Полный адрес HTTP-сервера, например `127.0.0.1:8081` |
| `SHUTDOWN_TIMEOUT` | Таймаут остановки, например `10s` |
| `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE` | Соответствующие поля PostgreSQL |
| `DATABASE_URL` | Строка подключения вместо полей PostgreSQL в YAML |

При заданном `DATABASE_URL` поля PostgreSQL из YAML не используются. Параметры,
явно заданные в строке подключения, имеют приоритет над `PG*`; для отсутствующих
параметров pgx может использовать соответствующие переменные окружения.
`DATABASE_URL` и другие стандартные переменные окружения при локальном запуске
сервиса задаются в PowerShell; в Compose передаются только переменные,
перечисленные в `docker-compose.yaml`. Секреты настоящего окружения храните вне
репозитория, например в переменных окружения или в отдельном файле конфигурации.

## Ручная проверка GraphQL

Сначала определите функцию для HTTP-запросов. Заголовок `X-User-ID` задаёт
демонстрационного пользователя для мутаций. Это не аутентификация: клиент может
подставить другой ID.

```powershell
function Invoke-GraphQL($Query, $Variables = @{}) {
    $body = @{
        query = $Query
        variables = $Variables
    } | ConvertTo-Json -Depth 10

    Invoke-RestMethod `
        -Uri 'http://localhost:8081/graphql' `
        -Method Post `
        -ContentType 'application/json' `
        -Headers @{ 'X-User-ID' = 'alice' } `
        -Body $body
}
```

При ошибках GraphQL ответ содержит поле `errors`; проверяйте его, прежде чем
использовать ID из ответа.

### Создание поста и комментариев

```powershell
$created = Invoke-GraphQL `
    'mutation($input: CreatePostInput!) {
        createPost(input: $input) { id title commentsEnabled }
    }' `
    @{ input = @{ title = 'Первый пост'; content = 'Текст поста' } }
$created | ConvertTo-Json -Depth 10
$postId = $created.data.createPost.id

$comment = Invoke-GraphQL `
    'mutation($input: CreateCommentInput!) {
        createComment(input: $input) { id postId parentId text authorId }
    }' `
    @{ input = @{ postId = $postId; text = 'Первый комментарий' } }
$comment | ConvertTo-Json -Depth 10
$commentId = $comment.data.createComment.id

Invoke-GraphQL `
    'mutation($input: CreateCommentInput!) {
        createComment(input: $input) { id parentId text }
    }' `
    @{ input = @{ postId = $postId; parentId = $commentId; text = 'Ответ' } } |
    ConvertTo-Json -Depth 10
```

### Чтение и пагинация

```powershell
Invoke-GraphQL `
    '{ posts(first: 10) {
        nodes {
            id title
            comments(first: 10) {
                nodes { id text replies(first: 10) { nodes { id text } } }
                pageInfo { endCursor hasNextPage }
            }
        }
        pageInfo { endCursor hasNextPage }
    } }' |
    ConvertTo-Json -Depth 15
```

Для следующей страницы передайте `endCursor` в `after` соответствующего списка,
например `posts(first: 10, after: "...")`. Не переносите курсор между разными
постами, родительскими комментариями и видами списков. Допустимый `first` — от
1 до 100. Если ветка глубже одного запроса, начните новый запрос с `comment(id:)`.

### Запрет комментирования

Автор поста может отключить комментарии:

```powershell
Invoke-GraphQL `
    'mutation($id: ID!) {
        setCommentsEnabled(postId: $id, enabled: false) { id commentsEnabled }
    }' `
    @{ id = $postId } | ConvertTo-Json -Depth 10
```

Повторное создание комментария к этому посту вернёт ошибку с кодом
`COMMENTS_DISABLED`. Чтобы вернуть возможность комментировать, выполните ту же
мутацию с `enabled: true`. При использовании другого `X-User-ID` изменение
настроек вернёт `FORBIDDEN`.

### Проверка подписки

1. В WebSocket-клиенте подключитесь к `ws://localhost:8081/graphql`, указав
   подпротокол `graphql-transport-ws`.
2. Отправьте `{"type":"connection_init"}` и дождитесь `connection_ack`.
3. Отправьте сообщение ниже, подставив значение `$postId` вместо `POST_ID`:

   ```json
   {"id":"comments","type":"subscribe","payload":{"query":"subscription($postId:ID!){commentAdded(postId:$postId){id postId parentId text}}","variables":{"postId":"POST_ID"}}}
   ```

4. Убедитесь, что комментарии к посту разрешены, и создайте комментарий через
   HTTP-мутацию из примера выше. В WebSocket придёт сообщение `next` с новым
   комментарием. Для отмены отправьте `{"id":"comments","type":"complete"}`;
   на серверный `ping` отвечайте `{"type":"pong"}`.

Доставка подписок действует внутри одного процесса и не хранит историю событий.
Подробнее о протоколе, ограничениях и операциях — в [README API](internal/api/README.md).

## Остановка и запуск без PostgreSQL

```powershell
docker compose down
```

Эта команда останавливает контейнеры и сохраняет том с данными PostgreSQL.

Для режима памяти без PostgreSQL сначала освободите порт `8081`, затем запустите:

```powershell
docker compose run --build --rm --no-deps --service-ports -e STORAGE=memory app
```

Остановите процесс через `Ctrl+C`. Данные в памяти исчезают при остановке.
При обычном `docker compose up` PostgreSQL запускается как зависимость даже при
`STORAGE=memory`.

При установленном Go версии из [go.mod](go.mod) приложение можно запускать и без
контейнера приложения:

```powershell
$env:STORAGE = 'memory'
go run ./cmd/service
```

## Автоматические тесты

Нужен Go версии, указанной в [go.mod](go.mod). Из корня проекта:

```powershell
go test ./... -count=1
go vet ./...
```

Без `TEST_DATABASE_URL` тесты, которым нужен реальный PostgreSQL, будут
пропущены. Для их запуска используйте отдельную тестовую БД с правом создавать
схемы. Например, поднимите временную БД на порту `5433`:

```powershell
docker run -d --rm --name ozon-test-db `
    -e POSTGRES_USER=ozon_test -e POSTGRES_PASSWORD=ozon_test `
    -e POSTGRES_DB=ozon_test -p 127.0.0.1:5433:5432 postgres:17-alpine

# Повторите команду, пока она не сообщит, что сервер принимает подключения.
docker exec ozon-test-db pg_isready -U ozon_test -d ozon_test

$env:TEST_DATABASE_URL = 'postgres://ozon_test:ozon_test@localhost:5433/ozon_test?sslmode=disable'
go test ./... -count=1 -timeout=3m
```

Интеграционные тесты создают отдельные схемы и удаляют их после проверки. В конце
остановите временный контейнер и уберите переменную из текущего сеанса:

```powershell
docker stop ozon-test-db
Remove-Item Env:TEST_DATABASE_URL
```

Для дополнительной проверки гонок выполните `go test -race ./...`. Нужны включённый
CGO и совместимый C-компилятор; в CI тесты запускаются с PostgreSQL и race detector.
Настройки сервера, отдельный запуск миграций и другие варианты запуска описаны в
[README сервиса](cmd/service/README.md).
