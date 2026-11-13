# GraphQL API

Пакет `api/handler` предоставляет `http.Handler` поверх usecase. Транспорт использует
существующие DTO; домен и usecase не зависят от GraphQL.

Код разделён по ответственности:

- `internal/api`: конфигурация, контекст и алиасы интерфейсов usecase.
- `internal/api/handler`: сборка обработчика, проверка HTTP-запросов, маршрутизация WebSocket и HTTP-ошибки.
- `internal/api/sandbox`: страница Apollo Sandbox для локального тестирования.
- `internal/infrastructure/middleware`: request ID, демонстрационный actor ID и HTTP-таймаут.
- `internal/infrastructure/pubsub`: хаб событий с ограниченными очередями и числом подписок.
- `internal/usecase/subscription`: проверка существования поста, подписка и преобразование событий в DTO.
- `internal/graphql`: схема `schema.graphqls`, резолверы и настройка gqlgen в `server.go`.
- `internal/graphql/loader`: пакетная загрузка, ключи страниц и кэш одной операции.
- `internal/graphql/gqlerrors`: преобразование ошибок в безопасные GraphQL-ответы.
- `internal/graphql/limits`: проверка глубины и стоимости операций.
- `internal/graphql/ws`: WebSocket-адаптер с ограничением времени записи и остановкой соединений.
- `internal/graphql/generated`: сгенерированный исполнитель схемы.
- `internal/requestctx`: actor ID и request ID, общие для HTTP и GraphQL.

Контракты резолверов находятся в `internal/graphql/deps.go`, узкие интерфейсы чтения —
в `internal/graphql/loader/deps.go`. В обоих файлах есть команды генерации Mockery;
моки располагаются в подпакетах `mocks` рядом с соответствующими интерфейсами.
`internal/api/deps.go` сохраняет публичные имена интерфейсов через алиасы.
Пакеты GraphQL не импортируют `api`; общий контекст находится в `internal/requestctx`.

`NewHandler` подключает `RequestID` → `DemoActor` → `Timeout` перед проверкой HTTP-запроса
и исполнением GraphQL. `RequestID` генерирует новый UUID, записывает его в контекст и
`X-Request-ID` ответа. `DemoActor` читает `X-User-ID`, сохраняя приоритет доверенного
контекста. `Timeout` ограничивает ожидание ответа даже при зависимости, игнорирующей
отмену контекста. Тело ошибки таймаута задаётся в `api`, поэтому пакет middleware
не зависит от GraphQL. Этот таймаут буферизует ответ и предназначен для обычных
HTTP-операций. WebSocket upgrade проходит через `RequestID` и `DemoActor`, а затем
напрямую в GraphQL-сервер. Query/mutation по WebSocket получают отдельный deadline
операции, подписка живёт до отмены, отключения клиента или остановки сервера.

## Подключение

После создания репозиториев:

```
hub, err := pubsub.New(pubsub.Config{BufferSize: 64, MaxSubscriptions: 1024})
if err != nil {
    return err
}
defer hub.Close()
posts := postusecase.New(postRepository)
comments := commentusecase.New(commentRepository, hub)
subscriptions := subscription.New(posts, hub)
graph, err := handler.NewHandler(posts, comments, subscriptions, api.Config{})
if err != nil {
    return err
}
defer graph.Close()
mux := http.NewServeMux()
mux.Handle("/graphql", graph)
server := &http.Server{
    Addr:              ":8081",
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```

Готовая сборка зависимостей и запуск HTTP находятся в `cmd/service`.
Репозитории PostgreSQL и создание общего пула описаны в
[README PostgreSQL](../repositories/postgres/README.md), а реализация в памяти — в
[README memory](../repositories/memory/README.md). Один и тот же `hub` нужно передать
в usecase создания комментария и в usecase подписки. `handler.NewHandler` принимает
usecase подписок третьим аргументом и возвращает `*handler.Handler` с методом `Close()`.

При остановке вызвать `graph.Close()`, затем `server.Shutdown(ctx)` и `hub.Close()`.
`graph.Close()` отменяет активные WebSocket-соединения, включая ожидающие
`connection_init`; один `http.Server.Shutdown` не закрывает hijacked-сокеты.

Поддерживаются POST с `Content-Type: application/json`, GET для query и OPTIONS.
Мутации через GET и массивы HTTP batch-запросов не принимаются. Introspection включена.
Для подписок поддерживается GET upgrade на том же `/graphql`, подпротокол строго
`graphql-transport-ws`. Подписки через обычный HTTP возвращают `BAD_USER_INPUT`.

## Идентичность

Чтение публичное. Мутации требуют actor ID. Для демонстрации он передаётся заголовком
`X-User-ID: alice`. Клиент не передаёт автора в GraphQL input. Право менять
`commentsEnabled` проверяется usecase/репозиторием.

`X-User-ID` не подтверждает личность: заголовок доверенный только в демонстрационном
режиме. Middleware настоящей аутентификации должен проверять запрос и устанавливать
идентичность через `api.WithActor(ctx, actorID)`; значение из контекста имеет приоритет
над заголовком. Пустое значение, явно установленное через `WithActor`, запрещает
использовать заголовок как запасной источник.

## Операции

Создание поста (по умолчанию комментарии разрешены):

```graphql
mutation {
  createPost(input: {title: "Первый пост", content: "Содержание"}) {
    id
    commentsEnabled
    createdAt
  }
}
```

Список постов, корни обсуждений и прямые ответы:

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

Создание комментария; `parentId: null` означает корень, UUID родителя — ответ:

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

Переключение возможности комментирования:

```graphql
mutation SetComments($postId: ID!, $enabled: Boolean!) {
  setCommentsEnabled(postId: $postId, enabled: $enabled) {
    id
    commentsEnabled
  }
}
```

Для продолжения глубокой ветки можно начать новый запрос с любого комментария:

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

`first` по умолчанию 20, диапазон 1–100. Явный `null` для `first` и
`commentsEnabled` при создании запрещён. Курсор берётся из `pageInfo.endCursor`
соответствующего списка; переносить его между постами, родителями и видами списков нельзя.
Пустой `after: ""` является ошибкой. Отсутствующие пост/комментарий по корректному ID
дают `null` без ошибки. Пустой список возвращается как `nodes: []`.

## Ошибки и ограничения

Прикладные ошибки содержат `extensions.code` и `extensions.requestId`.
ID запроса также возвращается в `X-Request-ID`; внутренние ошибки записываются
в `Config.Logger`, а клиент получает безопасное сообщение.

- `BAD_USER_INPUT`: некорректные ID, курсоры, размеры страниц, текст и другие входные значения.
- `UNAUTHENTICATED`: actor ID отсутствует.
- `FORBIDDEN`: изменение настроек чужого поста.
- `NOT_FOUND`: отсутствует объект, необходимый для мутации/страницы.
- `COMMENTS_DISABLED`, `PARENT_POST_MISMATCH`: правила размещения комментария.
- `INTERNAL`: ошибка хранилища, неизвестная ошибка или panic.
- `GRAPHQL_PARSE_FAILED`, `GRAPHQL_VALIDATION_FAILED`: ошибки GraphQL-документа/переменных.
- `QUERY_TOO_DEEP`, `QUERY_TOO_COMPLEX`, `PAYLOAD_TOO_LARGE`, `TIMEOUT`: превышение ограничений.
- `CANCELED`: отмена операции, если ещё возможно вернуть ответ.
- `RESOURCE_EXHAUSTED`: достигнут лимит активных подписок.
- `UNAVAILABLE`: хаб подписок остановлен.

Нулевые поля `Config` включают значения по умолчанию: 1 MiB входного тела/строки query,
5 секунд на HTTP-операцию, 20 уровней полей и стоимость 10 000. HTTP-таймаут отменяет
контекст usecase; при превышении времени ожидания ответа возвращает HTTP 503.
Для медленного чтения из сети дополнительно задаются таймауты `http.Server`, как в примере.

Стоимость страниц учитывает `first × стоимость вложенных полей`; aliases и fragments
учитываются, `@skip`/`@include` применяются с фактическими переменными. Вычисление
останавливается до переполнения чисел. Развёртывание fragments также имеет ненулевую
стоимость, даже если все поля пропущены. Проверки выполняются до usecase.
Лимит глубины ограничивает запрос, а не глубину хранимой ветки.

Парсер ограничен 10 000 токенами. Генерируемый исполнитель использует максимум
100 параллельных обработчиков дочерних полей. Фиксированные значения заданы в
`internal/graphql/server.go` и `gqlgen.yml`.

## Пакетная загрузка

Загрузчики создаются для каждой операции, объединяют запросы в окне 5 мс и передают
не более 100 ключей в одном вызове. ID загружаются через `GetMany`, вложенные страницы —
через `ListRootsBatch`/`ListRepliesBatch`. Ключ страницы включает ID владельца,
`first`, наличие и значение `after`. Кэш не разделяется между HTTP-запросами;
после каждой успешной мутации он очищается перед чтением вложенных полей.

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

Подписка публичная, как и чтение постов. Некорректный UUID возвращает `BAD_USER_INPUT`,
отсутствующий пост — `NOT_FOUND`. Запрет комментирования не запрещает подписку.
События содержат сохранённые корни и ответы и публикуются usecase после успешной
записи. Ошибка репозитория не порождает событие; отмена запроса после commit не отменяет
публикацию. GraphQL не публикует повторное событие поверх usecase.

Для ручной проверки открыть WebSocket `ws://localhost:8081/graphql` с подпротоколом
`graphql-transport-ws`, отправить `{"type":"connection_init"}` и дождаться
`connection_ack`. Затем отправить:

```json
{"id":"comments","type":"subscribe","payload":{"query":"subscription($postId:ID!){commentAdded(postId:$postId){id parentId text}}","variables":{"postId":"UUID существующего поста"}}}
```

Создать комментарий HTTP-мутацией: сервер пришлёт `next` с `payload.data.commentAdded`.
Для отмены отправить `{"id":"comments","type":"complete"}`. На `ping` отвечать `pong`.
Поле `connection_init.payload` не устанавливает actor: идентичность задаётся HTTP
middleware при upgrade. Браузер может подписываться анонимно, а мутации отправлять POST.

Сервер ждёт `connection_init` до 5 секунд, отправляет ping каждые 15 секунд и
закрывает соединение без pong; запись ограничена 10 секундами. `MaxBodyBytes` также
ограничивает размер WebSocket-сообщения. По умолчанию разрешены same-origin и клиенты
без Origin; дополнительные браузерные origins задаются через `WebsocketOrigins`
(например, `[]string{"localhost:3000"}`). Глубина и сложность проверяются при запуске
каждой операции, включая подписку.

Каждое событие получает новый набор загрузчиков в `AroundResponses` перед исполнением
вложенных резолверов. Используется обычный канал подписки; контекст события наследует
actor, request ID и отмену подписки. Поэтому вложенные `replies` не используют кэш
предыдущего события. WebSocket-транспорт
зарегистрирован перед HTTP ([документация gqlgen](https://gqlgen.com/recipes/subscriptions/)).

Хаб по умолчанию допускает 1024 подписки с очередью из 64 событий на каждую.
Настройки передаются через `pubsub.Config`. При переполнении медленная подписка
закрывается, её необработанные события отбрасываются; остальные подписчики продолжают
работать. После `complete` или разрыва связи клиент перечитывает нужные страницы.
Доставка действует внутри одного процесса: нет истории, replay, exactly-once и
согласованного снимка между первым чтением и регистрацией подписки. Сбой между commit
и publish может потерять событие; межпроцессная доставка потребует внешнего брокера.

## Генерация и проверки

Из корня репозитория:

```sh
go generate ./internal/graphql/... ./internal/usecase/subscription/...
go test ./internal/api/... ./internal/graphql/... -count=1
go test ./...
go vet ./...
```

Версии gqlgen и Mockery закреплены как Go tools в `go.mod`. При генерации моков
каждый пакет исключает сгенерированный код и подпакеты с собственными интерфейсами.
Файлы в `generated` и `mocks` вручную не редактируются. Конфигурация gqlgen находится
в корневом `gqlgen.yml`; директива в `internal/graphql/resolver.go` запускает генератор из корня.

Тесты расположены рядом с проверяемым кодом. В `internal/api` проверяются HTTP-транспорт,
конфигурация и таймауты; в `internal/graphql` — схема, резолверы, пакетная загрузка
1/10/50 родителей, изоляция кэша между запросами и его очистка после мутаций.
Подпакеты `limits`, `loader` и `gqlerrors` содержат прямые тесты ограничений,
загрузчиков и преобразования ошибок. Вспомогательные функции находятся в локальных
`*_test.go`. Интеграционный тест в `internal/api` соединяет HTTP с настоящими usecase
и моками репозиториев, включая создание корня/ответа и переход по курсору.
В `internal/infrastructure/middleware` отдельно проверяются request ID, приоритет actor
и таймаут. Тесты таймаута используют виртуальное время `testing/synctest`.
Тесты хаба и usecase подписки лежат рядом с реализациями. WebSocket-интеграция в
`internal/api/subscription_test.go` проверяет handshake, доставку после записи,
нескольких клиентов, свежий кэш события, ошибки, отмену, остановку, Origin и лимит сообщения.
