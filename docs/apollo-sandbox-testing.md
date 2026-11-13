# Проверка приложения в Apollo Sandbox

Инструкция покрывает посты, вложенные комментарии, пагинацию, права и подписки.
Запросы создают данные в выбранном хранилище. Используйте локальную тестовую
среду: данные PostgreSQL сохраняются после перезапуска, а данные в памяти исчезают.

## 1. Подготовка

Запустите Docker Desktop, затем из корня проекта в PowerShell выполните:

```powershell
docker compose up --build -d --wait
docker compose ps
```

Откройте [Apollo Sandbox](http://localhost:8081/sandbox). Для загрузки интерфейса
нужен интернет. Endpoint должен быть `http://localhost:8081/graphql`, а слева
должна загрузиться схема. Если этого не произошло, проверьте
`docker compose logs app postgres` и `sandbox.enabled: true` в YAML-конфигурации.

В нижней части вкладки **Operation** откройте **Headers**, нажмите **New header**
и добавьте `X-User-ID` со значением `alice`. Выполняйте запросы кнопкой **Run**.
Если указаны **Variables**, вставляйте JSON целиком во вкладку Variables.
Прикладные ошибки находятся в `errors[].extensions.code`; HTTP-статус может быть 200.

`alice` и `bob` — демонстрационные идентификаторы, не учётные записи. Заголовок
можно подделать: приложение не подтверждает личность пользователя.

## 2. Подготовьте посты

С заголовком `X-User-ID: alice` выполните:

```graphql
mutation CreateTestPosts {
  postA: createPost(input: {title: "Тест A: обсуждение", content: "Пост для проверки комментариев"}) {
    id title authorId commentsEnabled
  }
  postB: createPost(input: {title: "Тест B: другой пост", content: "Пост для проверки изоляции"}) {
    id title authorId commentsEnabled
  }
  closedPost: createPost(input: {title: "Тест C: закрыт", content: "Пост без комментариев", commentsEnabled: false}) {
    id title authorId commentsEnabled
  }
}
```

Ожидается: три поста с автором `alice`; у A и B `commentsEnabled: true`, у C —
`false`. Скопируйте UUID из ответа и сохраните как `POST_A`, `POST_B`, `POST_C`.
UUID генерируются сервером; во всех примерах заменяйте обозначения своими ID.

```graphql
query ListPosts {
  posts(first: 10) {
    nodes { id title authorId commentsEnabled }
    pageInfo { endCursor hasNextPage }
  }
}
```

В свежем хранилище видны созданные посты; в непустом они могут находиться на
следующей странице. Порядок при совпадении времени зависит от UUID.

## 3. Создайте ветку комментариев

Оставьте заголовок `alice` и выполните:

```graphql
mutation CreateRoots($postId: ID!) {
  root1: createComment(input: {postId: $postId, text: "Корень 1"}) { id postId parentId text }
  root2: createComment(input: {postId: $postId, text: "Корень 2"}) { id postId parentId text }
  root3: createComment(input: {postId: $postId, text: "Корень 3"}) { id postId parentId text }
}
```

Variables:

```json
{"postId":"POST_A"}
```

Ожидается: три разных ID, одинаковый `postId` и `parentId: null`. Сохраните
`root1.id` как `ROOT_1`. Создайте три прямых ответа:

```graphql
mutation CreateReplies($postId: ID!, $parentId: ID!) {
  reply1: createComment(input: {postId: $postId, parentId: $parentId, text: "Ответ 1"}) { id parentId text }
  reply2: createComment(input: {postId: $postId, parentId: $parentId, text: "Ответ 2"}) { id parentId text }
  reply3: createComment(input: {postId: $postId, parentId: $parentId, text: "Ответ 3"}) { id parentId text }
}
```

Variables:

```json
{"postId":"POST_A","parentId":"ROOT_1"}
```

У всех ответов `parentId` равен `ROOT_1`. Сохраните `reply1.id` как `REPLY_1` и
создайте ответ на ответ:

```graphql
mutation CreateNestedReply($postId: ID!, $parentId: ID!) {
  createComment(input: {postId: $postId, parentId: $parentId, text: "Ответ второго уровня"}) {
    id postId parentId text
  }
}
```

Variables:

```json
{"postId":"POST_A","parentId":"REPLY_1"}
```

Ожидается: `parentId` равен `REPLY_1`. Сохраните ID как `NESTED_1`. Повторяя
мутацию с ID последнего ответа, можно создавать более глубокую ветку.

## 4. Прочитайте пост и глубокую ветку

```graphql
query ReadPost($postId: ID!) {
  post(id: $postId) {
    id title
    comments(first: 10) {
      nodes {
        id text parentId
        replies(first: 10) {
          nodes { id text parentId replies(first: 10) { nodes { id text parentId } } }
        }
      }
    }
  }
}
```

Variables:

```json
{"postId":"POST_A"}
```

Ожидается: в `comments` — три корня; ответы находятся в `replies` своего
родителя, а «Ответ второго уровня» — в `replies` комментария `REPLY_1`.

Чтобы продолжить чтение глубокой ветки, начните запрос с найденного комментария:

```graphql
query ReadComment($id: ID!) {
  comment(id: $id) { id text parentId replies(first: 10) { nodes { id text parentId } } }
}
```

Variables:

```json
{"id":"REPLY_1"}
```

В ответе должен быть `NESTED_1`. Хранимая глубина не ограничена, но глубина и
сложность одного GraphQL-запроса ограничены; `comment(id:)` позволяет продолжить
чтение отдельным запросом.

## 5. Проверьте курсорную пагинацию

```graphql
query RootPage($postId: ID!, $after: String) {
  post(id: $postId) {
    comments(first: 2, after: $after) {
      nodes { id text }
      pageInfo { endCursor hasNextPage }
    }
  }
}
```

Variables для первой страницы:

```json
{"postId":"POST_A","after":null}
```

Ожидается: два корня, `hasNextPage: true` и непустой `endCursor`. Скопируйте его,
оставьте запрос прежним и замените Variables:

```json
{"postId":"POST_A","after":"СКОПИРОВАННЫЙ_END_CURSOR"}
```

Ожидается: оставшийся корень, `hasNextPage: false`, без повторения ID первой
страницы. Подставьте новый `endCursor`: следующая страница пуста, её `endCursor`
равен `null`. Это верно для нового поста с тремя корнями без дополнительных вставок.

Аналогично проверьте три ответа `ROOT_1`:

```graphql
query ReplyPage($parentId: ID!, $after: String) {
  comment(id: $parentId) {
    replies(first: 2, after: $after) {
      nodes { id text }
      pageInfo { endCursor hasNextPage }
    }
  }
}
```

Variables:

```json
{"parentId":"ROOT_1","after":null}
```

Ожидаются две страницы с двумя и одним ответом. Для второй подставьте курсор
первой. Курсор одного списка нельзя использовать для другого поста или родителя.
`first: 0`, `first: 101` или повреждённый курсор возвращают `BAD_USER_INPUT`.

## 6. Проверьте длину и валидацию

Используйте операцию, подставляя Variables из таблицы:

```graphql
mutation AddText($postId: ID!, $parentId: ID, $text: String!) {
  createComment(input: {postId: $postId, parentId: $parentId, text: $text}) { id text }
}
```

| Variables | Ожидаемый результат |
| --- | --- |
| `{"postId":"POST_A","text":"Привет, мир!"}` | Комментарий создан. |
| `{"postId":"POST_A","text":""}` | `BAD_USER_INPUT`. |
| `{"postId":"POST_A","text":"   "}` | `BAD_USER_INPUT`. |
| `{"postId":"не-uuid","text":"Текст"}` | `BAD_USER_INPUT`. |
| `{"postId":"11111111-1111-4111-8111-111111111111","text":"Текст"}` | `NOT_FOUND`, если UUID отсутствует. |
| `{"postId":"POST_A","parentId":"11111111-1111-4111-8111-111111111111","text":"Текст"}` | `NOT_FOUND`, если родитель отсутствует. |
| `{"postId":"POST_B","parentId":"ROOT_1","text":"Чужой родитель"}` | `PARENT_POST_MISMATCH`. |
| `{"postId":"POST_C","text":"Закрытый пост"}` | `COMMENTS_DISABLED`. |

Замените обозначения своими UUID. Ошибки не должны создавать комментарии. Для
отсутствующего поста **при чтении** `post(id:)` возвращается `null`; `NOT_FOUND`
ожидается при создании комментария к отсутствующему посту.

Проверьте границу 2000 Unicode-кодов. В PowerShell подставьте UUID и скопируйте
готовый JSON во вкладку Variables:

```powershell
$postId = 'POST_A' # замените на UUID поста
@{ postId = $postId; text = ('я' * 2000) } | ConvertTo-Json -Compress | Set-Clipboard
```

С этими Variables `AddText` создаёт комментарий. Повторите с `('я' * 2001)` —
ожидается `BAD_USER_INPUT`. Считаются Unicode code points, а не байты или
обязательно видимые символы: один видимый символ может состоять из нескольких.

## 7. Проверьте права и запрет комментариев

```graphql
mutation SetComments($postId: ID!, $enabled: Boolean!) {
  setCommentsEnabled(postId: $postId, enabled: $enabled) { id commentsEnabled }
}
```

Variables:

```json
{"postId":"POST_A","enabled":false}
```

1. Установите заголовок `X-User-ID: bob` и выполните операцию. Ожидается
   `FORBIDDEN`; при чтении поста флаг остаётся `true`.
2. Верните `alice` и повторите операцию. Ожидается `commentsEnabled: false`.
3. Выполните `AddText` для нового корня и затем с `parentId: ROOT_1`. Оба
   запроса возвращают `COMMENTS_DISABLED`; существующие комментарии видны.
4. Выполните `SetComments` с `{"postId":"POST_A","enabled":true}`. Флаг снова
   `true`, и новый комментарий успешно создаётся.
5. Отключите заголовок `X-User-ID` и попробуйте создать пост или комментарий.
   Ожидается `UNAUTHENTICATED`. Чтение работает без заголовка.

## 8. Проверьте GraphQL Subscriptions

Это дополнительное требование задания. Верните `X-User-ID: alice`, убедитесь,
что комментарии к A разрешены, и откройте Sandbox в двух вкладках браузера:
одну для подписки, вторую для мутаций.

В первой вкладке откройте **Connection settings**:

- Subscriptions URL: `ws://localhost:8081/graphql`;
- Implementation: `graphql-ws` (подпротокол `graphql-transport-ws`).

Сохраните настройки и выполните:

```graphql
subscription NewComments($postId: ID!) {
  commentAdded(postId: $postId) { id postId parentId text authorId }
}
```

Variables:

```json
{"postId":"POST_A"}
```

Дождитесь состояния подключения и **Listening**. Во второй вкладке создайте
комментарий к B через `AddText`: подписка A не должна его получить. Затем создайте
комментарий к A с текстом `Событие A`. В первой вкладке появится событие с этим
текстом, ID поста A и `parentId: null`. Создайте ответ с `parentId: ROOT_1`:
подписка A также должна получить его.

Пока подписка активна, отключите комментарии к A через `SetComments` и попробуйте
создать комментарий. Мутация вернёт `COMMENTS_DISABLED`, нового события быть не
должно. Верните `enabled: true`, остановите подписку кнопкой **Stop listening**,
создайте ещё один комментарий и убедитесь, что новые события не поступают.

Доставка работает внутри одного процесса. Истории пропущенных событий и replay
нет; при переполнении очереди медленная подписка закрывается. Подробнее — в
[README API](../internal/api/README.md).

## 9. Проверки вне Sandbox

Ручные сценарии не доказывают отсутствие N+1-запросов или производительность под
нагрузкой. Для этого нужны автоматические и нагрузочные проверки. В проекте есть
тесты домена, usecase, HTTP/WebSocket и хранилищ; команды и подготовка отдельной
тестовой базы описаны в [корневом README](../README.md#автоматические-тесты).

После проверки остановите Compose командой `docker compose down`. Данные тома
PostgreSQL сохранятся. Не удаляйте том, если в нём есть нужные данные.
