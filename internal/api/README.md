# GraphQL API

Пакет `api` предоставляет `http.Handler` поверх usecase. Транспорт использует
существующие DTO; домен и usecase не зависят от GraphQL.

Код разделён по ответственности:

- `internal/api`: публичный `NewHandler`, конфигурация и HTTP-ограничения.
- `internal/infrastructure/middleware`: request ID, демонстрационный actor ID и HTTP-таймаут.
- `internal/graphql`: схема `schema.graphqls`, резолверы и настройка gqlgen в `server.go`.
- `internal/graphql/loader`: пакетная загрузка, ключи страниц и кэш одной операции.
- `internal/graphql/gqlerrors`: преобразование ошибок в безопасные GraphQL-ответы.
- `internal/graphql/limits`: проверка глубины и стоимости операций.
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
HTTP-операций; для будущих WebSocket-подписок потребуется отдельная цепочка.

## Подключение

После создания репозиториев и usecase:

```
graph, err := api.NewHandler(postUseCase, commentUseCase, api.Config{})
if err != nil {
    return err
}
mux := http.NewServeMux()
mux.Handle("/graphql", graph)
server := &http.Server{
    Addr:              ":8080",
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```

Это пример сборки зависимостей, а не готовый исполняемый сервис: реализации хранилища
и запуск в `cmd/service` добавляются отдельно. GraphQL subscriptions относятся к следующему
этапу и пока не включены в схему или HTTP-транспорты.

Поддерживаются POST с `Content-Type: application/json`, GET для query и OPTIONS.
Мутации через GET и массивы HTTP batch-запросов не принимаются. Introspection включена.

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

## Генерация и проверки

Из корня репозитория:

```sh
go generate ./internal/graphql/...
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
