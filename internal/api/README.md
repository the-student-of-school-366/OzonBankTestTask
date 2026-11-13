# GraphQL API

Пакет `api/handler` предоставляет `http.Handler` поверх usecase. Транспорт использует DTO. Домен и usecase не зависят от GraphQL.

Сборка приложения, включая этот обработчик, находится в `cmd/service` и выполняется через uber/fx.

## Состав

- `internal/api` — конфигурация, контекст и алиасы интерфейсов usecase.
- `internal/api/handler` — сборка обработчика, проверка HTTP, маршрутизация WebSocket и HTTP-ошибки.
- `internal/api/sandbox` — страница Apollo Sandbox.
- `internal/infrastructure/middleware` — идентификатор запроса, демонстрационный автор, журнал запроса и HTTP-таймаут.
- `internal/infrastructure/pubsub` — хаб событий с ограниченной очередью и числом подписок.
- `internal/broker` — контракт потока подписки.
- `internal/usecase/subscription` — проверка поста, подписка и преобразование события в DTO.
- `internal/graphql` — схема, резолверы и сборка исполнителя.
- `internal/graphql/loader` — пакетная загрузка и кэш одной операции.
- `internal/graphql/gqlerrors` — безопасные ответы об ошибках.
- `internal/graphql/limits` — глубина и стоимость операции.
- `internal/graphql/ws` — WebSocket-адаптер с ограничением записи и остановкой соединений.
- `internal/graphql/generated` — сгенерированный исполнитель схемы.
- `internal/requestctx` — автор и идентификатор запроса для HTTP и GraphQL.

Контракты резолверов лежат в `internal/graphql/deps.go`, интерфейсы чтения — в `internal/graphql/loader/deps.go`. Рядом с ними команды Mockery. Моки лежат в подпакетах `mocks`. `internal/api/deps.go` сохраняет публичные имена через алиасы. Пакеты GraphQL не импортируют `api`.

Запрос проходит так: `RequestID`, затем `AccessLog`, `DemoActor` и развилка транспорта. Обычный HTTP дополнительно проходит `Timeout` и проверку метода, типа и размера тела. `RequestID` создаёт новый UUID, кладёт его в контекст и в заголовок `X-Request-ID`. Входящий `X-Request-ID` не используется. `DemoActor` читает `X-User-ID`, если автор ещё не задан доверенным контекстом. `Timeout` ограничивает ожидание ответа и при превышении возвращает HTTP 503. Он буферизует ответ и применяется к обычным HTTP-операциям.

WebSocket upgrade идёт через `RequestID`, `AccessLog` и `DemoActor`, затем сразу в GraphQL-сервер. Query и mutation по WebSocket получают отдельный срок операции. Подписка живёт до отмены, отключения клиента или остановки сервера.

`NewHandler` возвращает `*handler.Handler` с методом `Close()`. При остановке сначала вызывается `graph.Close()`: он отменяет активные WebSocket-соединения, включая те, что ждут `connection_init`. Затем `server.Shutdown`. Один `Shutdown` не закрывает уже перехваченные сокеты. После остановки HTTP закрывается хаб подписок.

Поддерживаются POST с `Content-Type: application/json`, GET для query и OPTIONS. Мутации через GET и массивы HTTP-запросов не принимаются. Introspection включена. Подписка — это GET upgrade на том же `/graphql` с подпротоколом `graphql-transport-ws`. Подписка обычным HTTP возвращает `BAD_USER_INPUT`.

## Идентичность

Чтение публичное. Мутации требуют автора. Для демонстрации он передаётся заголовком `X-User-ID: alice`. В полях GraphQL автора нет. Право менять `commentsEnabled` проверяет usecase вместе с репозиторием.

Заголовок демонстрационный. Настоящая аутентификация должна проверить запрос и записать автора через `api.WithActor(ctx, actorID)`. Значение из контекста важнее заголовка. Пустое значение, явно записанное через `WithActor`, запрещает брать автора из заголовка.

## Операции

Создание поста. По умолчанию комментарии разрешены:

```graphql
mutation {
  createPost(input: {title: "Первый пост", content: "Содержание"}) {
    id
    commentsEnabled
    createdAt
  }
}
```

Список постов, корни и прямые ответы:

```graphql
query {
  posts(first: 10) {
    nodes {
      id
      title
      comments(first: 5) {
        nodes {
          id
          text
          replies(first: 5) {
            nodes { id text }
            pageInfo { endCursor hasNextPage }
          }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
    pageInfo { endCursor hasNextPage }
  }
}
```

Комментарий. `parentId: null` — корень, UUID родителя — ответ:

```graphql
mutation AddComment($postId: ID!, $parentId: ID, $text: String!) {
  createComment(input: {postId: $postId, parentId: $parentId, text: $text}) {
    id
    postId
    parentId
    authorId
    text
  }
}
```

Переключение комментариев:

```graphql
mutation SetComments($postId: ID!, $enabled: Boolean!) {
  setCommentsEnabled(postId: $postId, enabled: $enabled) {
    id
    commentsEnabled
  }
}
```

Продолжение глубокой ветки:

```graphql
query Continue($id: ID!, $after: String) {
  comment(id: $id) {
    id
    replies(first: 20, after: $after) {
      nodes { id text parentId }
      pageInfo { endCursor hasNextPage }
    }
  }
}
```

`first` по умолчанию 20, допустимый диапазон — от 1 до 100. Явный `null` для `first` и для `commentsEnabled` при создании запрещён. Курсор берётся из `pageInfo.endCursor` того же списка. Пустой `after: ""` — ошибка. Корректный идентификатор отсутствующего поста или комментария даёт `null` без ошибки. Пустой список приходит как `nodes: []`.

## Ошибки и ограничения

Прикладные ошибки содержат `extensions.code` и `extensions.requestId`. Идентификатор запроса также возвращается в `X-Request-ID`. Внутренние ошибки пишутся в `Config.Logger`. Клиент получает общий текст.

- `BAD_USER_INPUT` — некорректные идентификаторы, курсоры, размеры страниц, текст и другие входные значения.
- `UNAUTHENTICATED` — нет автора.
- `FORBIDDEN` — изменение настроек чужого поста.
- `NOT_FOUND` — нет объекта, нужного для мутации или страницы.
- `COMMENTS_DISABLED`, `PARENT_POST_MISMATCH` — правила размещения комментария.
- `INTERNAL` — ошибка хранилища, неизвестная ошибка или panic.
- `GRAPHQL_PARSE_FAILED`, `GRAPHQL_VALIDATION_FAILED` — документ или переменные GraphQL.
- `QUERY_TOO_DEEP`, `QUERY_TOO_COMPLEX`, `PAYLOAD_TOO_LARGE`, `TIMEOUT` — превышение ограничений.
- `CANCELED` — отмена операции, если ответ ещё можно вернуть.
- `RESOURCE_EXHAUSTED` — достигнут лимит активных подписок.
- `UNAVAILABLE` — хаб подписок остановлен.

Нулевые поля `Config` получают значения по умолчанию: 1 МиБ на тело или строку query, 5 секунд на HTTP-операцию, 20 уровней полей и стоимость 10 000. HTTP-таймаут отменяет контекст usecase. При превышении времени ожидания ответа возвращается HTTP 503. Таймауты `http.Server` дополнительно ограничивают медленное чтение из сети.

Стоимость страниц учитывает `first`, умноженный на стоимость вложенных полей. Учитываются aliases и fragments. `@skip` и `@include` применяются с фактическими переменными. Подсчёт останавливается до переполнения чисел. Разворот fragment имеет стоимость, даже если все его поля пропущены. Проверки выполняются до usecase. Лимит глубины ограничивает запрос, а не хранимую ветку.

Парсер ограничен 10 000 токенами. Исполнитель использует не больше 100 параллельных обработчиков дочерних полей. Эти значения заданы в `internal/graphql/server.go` и `gqlgen.yml`.

## Пакетная загрузка

Загрузчики создаются для каждой операции, объединяют запросы в окне 5 мс и передают не больше 100 ключей за вызов. Идентификаторы загружаются через `GetMany`, страницы — через `ListRootsBatch` и `ListRepliesBatch`. Ключ страницы включает идентификатор владельца, `first` и курсор `after`. Кэш не общий для HTTP-запросов. После успешной мутации он очищается перед чтением вложенных полей.

## Подписки

```graphql
subscription Comments($postId: ID!) {
  commentAdded(postId: $postId) {
    id
    postId
    parentId
    authorId
    text
    createdAt
    replies(first: 20) { nodes { id text } }
  }
}
```

Подписка публичная, как и чтение. Некорректный UUID возвращает `BAD_USER_INPUT`, отсутствующий пост — `NOT_FOUND`. Запрет комментирования подписку не отменяет. Событие содержит сохранённый корень или ответ и публикуется usecase после успешной записи. Ошибка репозитория события не создаёт. Отмена запроса после commit публикацию не отменяет. GraphQL повторно событие не публикует.

Для ручной проверки откройте WebSocket `ws://localhost:8081/graphql` с подпротоколом `graphql-transport-ws`, отправьте `{"type":"connection_init"}` и дождитесь `connection_ack`. Затем отправьте:

```json
{"id":"comments","type":"subscribe","payload":{"query":"subscription($postId:ID!){commentAdded(postId:$postId){id parentId text}}","variables":{"postId":"UUID существующего поста"}}}
```

Создайте комментарий HTTP-мутацией. Сервер пришлёт `next` с `payload.data.commentAdded`. Для отмены отправьте `{"id":"comments","type":"complete"}`. На `ping` отвечайте `pong`. Поле `connection_init.payload` автора не задаёт: его устанавливает HTTP middleware при upgrade. Браузер может подписаться без имени, а мутации отправлять отдельным POST.

Сервер ждёт `connection_init` до 5 секунд, отправляет ping каждые 15 секунд и закрывает соединение без pong. Запись ограничена 10 секундами. `MaxBodyBytes` ограничивает и размер WebSocket-сообщения. По умолчанию разрешены тот же origin и клиенты без заголовка Origin. Дополнительные origin задаются через `WebsocketOrigins`, например `[]string{"localhost:3000"}`. Глубина и сложность проверяются при запуске каждой операции, включая подписку.

Перед вложенными резолверами каждого события создаётся новый набор загрузчиков. Поэтому `replies` одного события не пользуются кэшем предыдущего. WebSocket-транспорт регистрируется раньше HTTP.

Хаб по умолчанию допускает 1024 подписки, у каждой очередь из 64 событий. При переполнении медленная подписка закрывается, её необработанные события отбрасываются, остальные продолжают работать. После `complete` или разрыва связи клиент заново читает нужные страницы. Доставка действует внутри одного процесса: нет истории, повтора и общего снимка между первым чтением и регистрацией подписки. Сбой между commit и публикацией может потерять событие.

## Генерация

Из корня репозитория:

```sh
go generate ./internal/graphql/... ./internal/usecase/... ./internal/broker/...
go test ./internal/api/... ./internal/graphql/... -count=1
```

Версии gqlgen и Mockery закреплены в `go.mod`. Файлы в `generated` и `mocks` вручную не редактируются. Конфигурация gqlgen лежит в корневом `gqlgen.yml`.
