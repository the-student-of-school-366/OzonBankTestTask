const state = {
  posts: [],
  postsPage: { endCursor: null, hasNextPage: false },
  post: null,
  branch: null,
  listeningId: null,
  socket: null,
  events: [],
};

const $ = (id) => document.getElementById(id);

const postFields = "id title content authorId commentsEnabled createdAt";
const commentFields = "id postId parentId authorId text createdAt";

function actor() {
  return $("actor").value.trim();
}

function pageSize() {
  return Number($("page-size").value);
}

function showNotice(kind, message, code) {
  const node = $("notice");
  node.hidden = false;
  node.className = `notice ${kind}`;
  node.replaceChildren();
  if (code) {
    const mark = document.createElement("strong");
    mark.textContent = code;
    node.append(mark, " ");
  }
  node.append(message);
}

function showError(errors) {
  const error = errors?.[0];
  const code = error?.extensions?.code || "ОШИБКА";
  showNotice("error", error?.message || "Запрос не выполнен", code);
}

function showOk(message) {
  showNotice("ok", message);
}

async function gql(query, variables) {
  const headers = { "Content-Type": "application/json" };
  const name = actor();
  if (name) headers["X-User-ID"] = name;
  let response;
  try {
    response = await fetch("/graphql", {
      method: "POST",
      headers,
      body: JSON.stringify({ query, variables }),
    });
  } catch {
    showNotice("error", "Сервис недоступен. Дождитесь запуска и повторите.");
    throw new Error("offline");
  }
  const body = await response.json().catch(() => null);
  if (!body) {
    showNotice("error", `Ответ не JSON, HTTP ${response.status}`);
    throw new Error("bad response");
  }
  if (body.errors?.length) {
    showError(body.errors);
    throw body.errors;
  }
  return body.data;
}

function stamp(value) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("ru-RU");
}

function pageVars(after) {
  return { first: pageSize(), after: after || null };
}

async function loadPosts(after) {
  const data = await gql(
    `query ($first: Int!, $after: String) {
      posts(first: $first, after: $after) {
        nodes { ${postFields} }
        pageInfo { endCursor hasNextPage }
      }
    }`,
    pageVars(after),
  );
  const page = data.posts;
  state.posts = after ? state.posts.concat(page.nodes) : page.nodes;
  state.postsPage = page.pageInfo;
  renderPosts();
}

async function openPost(id) {
  const data = await gql(
    `query ($id: ID!, $first: Int!, $after: String) {
      post(id: $id) {
        ${postFields}
        comments(first: $first, after: $after) {
          nodes { ${commentFields} replies(first: $first) { nodes { ${commentFields} } pageInfo { endCursor hasNextPage } } }
          pageInfo { endCursor hasNextPage }
        }
      }
    }`,
    { id, ...pageVars() },
  );
  state.post = data.post;
  renderThread();
  renderPosts();
  if (!data.post) showOk("Пост с таким идентификатором не найден. Это пустой ответ, не ошибка.");
}

async function moreComments() {
  const page = state.post.comments.pageInfo;
  const data = await gql(
    `query ($id: ID!, $first: Int!, $after: String) {
      post(id: $id) {
        comments(first: $first, after: $after) {
          nodes { ${commentFields} replies(first: $first) { nodes { ${commentFields} } pageInfo { endCursor hasNextPage } } }
          pageInfo { endCursor hasNextPage }
        }
      }
    }`,
    { id: state.post.id, ...pageVars(page.endCursor) },
  );
  const next = data.post.comments;
  state.post.comments.nodes.push(...next.nodes);
  state.post.comments.pageInfo = next.pageInfo;
  renderThread();
}

async function moreReplies(comment) {
  const data = await gql(
    `query ($id: ID!, $first: Int!, $after: String) {
      comment(id: $id) {
        replies(first: $first, after: $after) {
          nodes { ${commentFields} }
          pageInfo { endCursor hasNextPage }
        }
      }
    }`,
    { id: comment.id, ...pageVars(comment.replies.pageInfo.endCursor) },
  );
  if (!data.comment) {
    showOk("Комментарий больше не находится. Это пустой ответ, не ошибка.");
    return;
  }
  comment.replies.nodes.push(...data.comment.replies.nodes);
  comment.replies.pageInfo = data.comment.replies.pageInfo;
  renderThread();
  if (state.branch?.id === comment.id) renderBranch();
}

async function openBranch(id) {
  const data = await gql(
    `query ($id: ID!, $first: Int!, $after: String) {
      comment(id: $id) {
        ${commentFields}
        replies(first: $first, after: $after) {
          nodes { ${commentFields} }
          pageInfo { endCursor hasNextPage }
        }
      }
    }`,
    { id, ...pageVars() },
  );
  state.branch = data.comment;
  $("find-comment").value = id;
  renderBranch();
  if (!data.comment) showOk("Комментарий с таким идентификатором не найден. Это пустой ответ, не ошибка.");
}

function renderPosts() {
  const list = $("posts");
  list.replaceChildren();
  for (const post of state.posts) {
    const item = document.createElement("li");
    const button = document.createElement("button");
    button.type = "button";
    button.className = `post quiet${state.post?.id === post.id ? " selected" : ""}`;
    const title = document.createElement("strong");
    title.textContent = post.title;
    const meta = document.createElement("div");
    meta.className = "meta";
    meta.textContent = `${post.authorId} · ${post.id.slice(0, 8)} · ${post.commentsEnabled ? "открыты" : "закрыты"}`;
    button.append(title, meta);
    button.addEventListener("click", () => openPost(post.id).catch(() => {}));
    item.append(button);
    list.append(item);
  }
  $("more-posts").hidden = !state.postsPage.hasNextPage;
}

function commentView(comment, nested) {
  const item = document.createElement("li");
  item.className = "comment";
  const who = document.createElement("div");
  who.className = "who";
  who.textContent = `${comment.authorId} · ${stamp(comment.createdAt)}`;
  const text = document.createElement("p");
  text.textContent = comment.text;
  const actions = document.createElement("div");
  actions.className = "row";
  const reply = document.createElement("button");
  reply.type = "button";
  reply.className = "quiet";
  reply.textContent = "Ответить";
  reply.addEventListener("click", () => {
    $("parent-id").value = comment.id;
    $("comment-text").focus();
  });
  const branch = document.createElement("button");
  branch.type = "button";
  branch.className = "quiet";
  branch.textContent = "Ветка";
  branch.addEventListener("click", () => openBranch(comment.id).catch(() => {}));
  actions.append(reply, branch);
  item.append(who, text, actions);
  if (!nested && comment.replies) {
    const replies = document.createElement("ol");
    replies.className = "comments replies";
    for (const child of comment.replies.nodes) replies.append(commentView(child, true));
    item.append(replies);
    if (comment.replies.pageInfo.hasNextPage) {
      const more = document.createElement("button");
      more.type = "button";
      more.className = "quiet";
      more.textContent = "Ещё ответы";
      more.addEventListener("click", () => moreReplies(comment).catch(() => {}));
      item.append(more);
    }
  }
  return item;
}

function renderThread() {
  const view = $("post-view");
  const form = $("comment-form");
  const toggle = $("toggle-comments");
  view.replaceChildren();
  if (!state.post) {
    view.className = "empty";
    view.textContent = "Выберите пост слева или найдите его по идентификатору.";
    form.hidden = true;
    toggle.hidden = true;
    return;
  }
  view.className = "";
  const title = document.createElement("h3");
  title.textContent = state.post.title;
  const body = document.createElement("p");
  body.textContent = state.post.content;
  const meta = document.createElement("p");
  meta.className = "meta";
  meta.textContent = `${state.post.authorId} · ${stamp(state.post.createdAt)} · ${state.post.id}`;
  const flag = document.createElement("p");
  flag.textContent = state.post.commentsEnabled ? "Комментарии открыты." : "Комментарии закрыты. Уже написанное остаётся видимым.";
  const list = document.createElement("ol");
  list.className = "comments";
  for (const comment of state.post.comments.nodes) list.append(commentView(comment, false));
  view.append(title, body, meta, flag, list);
  if (state.post.comments.pageInfo.hasNextPage) {
    const more = document.createElement("button");
    more.type = "button";
    more.className = "quiet";
    more.textContent = "Ещё комментарии";
    more.addEventListener("click", () => moreComments().catch(() => {}));
    view.append(more);
  }
  form.hidden = false;
  toggle.hidden = false;
  toggle.textContent = state.post.commentsEnabled ? "Закрыть комментарии" : "Открыть комментарии";
}

function renderBranch() {
  const root = $("branch");
  root.replaceChildren();
  if (!state.branch) return;
  const card = document.createElement("article");
  card.className = "branch-card";
  const title = document.createElement("h3");
  title.textContent = "Продолжение ветки";
  const text = document.createElement("p");
  text.textContent = state.branch.text;
  const meta = document.createElement("p");
  meta.className = "meta";
  meta.textContent = `${state.branch.authorId} · ${state.branch.id}`;
  const list = document.createElement("ol");
  list.className = "comments";
  for (const comment of state.branch.replies.nodes) list.append(commentView(comment, true));
  card.append(title, text, meta, list);
  if (state.branch.replies.pageInfo.hasNextPage) {
    const more = document.createElement("button");
    more.type = "button";
    more.className = "quiet";
    more.textContent = "Ещё ответы ветки";
    more.addEventListener("click", async () => {
      await moreReplies(state.branch);
      const data = state.branch.replies;
      renderBranch();
      return data;
    });
    card.append(more);
  }
  root.append(card);
}

function renderEvents() {
  const list = $("events");
  list.replaceChildren();
  for (const event of state.events) {
    const item = document.createElement("li");
    const time = document.createElement("div");
    time.className = "event-time";
    time.textContent = stamp(event.createdAt);
    const text = document.createElement("p");
    text.textContent = event.text;
    const meta = document.createElement("div");
    meta.className = "meta";
    meta.textContent = event.parentId ? `ответ на ${event.parentId}` : "корень";
    item.append(time, text, meta);
    list.append(item);
  }
  $("listen-status").textContent = state.listeningId ? "Слушаем пост" : "Не слушаем";
}

function stopListening() {
  const socket = state.socket;
  state.socket = null;
  state.listeningId = null;
  if (socket) {
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ id: "comments", type: "complete" }));
    }
    socket.close();
  }
  renderEvents();
}

function listen() {
  if (!state.post) {
    showNotice("error", "Сначала выберите пост, который нужно слушать.");
    return;
  }
  stopListening();
  const postId = state.post.id;
  const protocol = location.protocol === "https:" ? "wss:" : "ws:";
  const socket = new WebSocket(`${protocol}//${location.host}/graphql`, "graphql-transport-ws");
  state.socket = socket;
  state.listeningId = postId;
  $("listen-status").textContent = "Подключение";
  socket.addEventListener("open", () => {
    if (state.socket !== socket) return;
    socket.send(JSON.stringify({ type: "connection_init" }));
  });
  socket.addEventListener("message", (event) => {
    if (state.socket !== socket) return;
    const message = JSON.parse(event.data);
    if (message.type === "ping") {
      socket.send(JSON.stringify({ type: "pong" }));
      return;
    }
    if (message.type === "connection_ack") {
      socket.send(JSON.stringify({
        id: "comments",
        type: "subscribe",
        payload: {
          query: `subscription ($postId: ID!) { commentAdded(postId: $postId) { ${commentFields} } }`,
          variables: { postId },
        },
      }));
      renderEvents();
      return;
    }
    if (message.type === "next" && message.payload?.data?.commentAdded) {
      state.events.unshift(message.payload.data.commentAdded);
      renderEvents();
      if (state.post?.id === postId) openPost(postId).catch(() => {});
      return;
    }
    if (message.type === "error") showError(message.payload);
  });
  socket.addEventListener("close", () => {
    if (state.socket !== socket) return;
    state.socket = null;
    state.listeningId = null;
    renderEvents();
    showNotice("error", "Подписка закрылась до остановки.");
  });
}

$("post-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const data = await gql(
    `mutation ($input: CreatePostInput!) { createPost(input: $input) { id } }`,
    {
      input: {
        title: $("post-title").value,
        content: $("post-content").value,
        commentsEnabled: $("comments-enabled").checked,
      },
    },
  ).catch(() => null);
  if (!data) return;
  $("post-form").reset();
  $("comments-enabled").checked = true;
  showOk("Пост опубликован.");
  await loadPosts().catch(() => {});
  await openPost(data.createPost.id).catch(() => {});
});

$("comment-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const parent = $("parent-id").value.trim();
  const input = { postId: state.post.id, text: $("comment-text").value };
  if (parent) input.parentId = parent;
  const data = await gql(
    `mutation ($input: CreateCommentInput!) { createComment(input: $input) { id parentId } }`,
    { input },
  ).catch(() => null);
  if (!data) return;
  $("comment-text").value = "";
  showOk(parent ? "Ответ добавлен." : "Комментарий добавлен.");
  await openPost(state.post.id).catch(() => {});
  if (state.branch && (state.branch.id === parent || state.branch.id === data.createComment.parentId)) {
    await openBranch(state.branch.id).catch(() => {});
  }
});

$("toggle-comments").addEventListener("click", async () => {
  const enabled = !state.post.commentsEnabled;
  const data = await gql(
    `mutation ($id: ID!, $enabled: Boolean!) { setCommentsEnabled(postId: $id, enabled: $enabled) { id } }`,
    { id: state.post.id, enabled },
  ).catch(() => null);
  if (!data) return;
  showOk(enabled ? "Комментарии открыты." : "Комментарии закрыты.");
  await openPost(state.post.id).catch(() => {});
  await loadPosts().catch(() => {});
});

$("reload-posts").addEventListener("click", () => loadPosts().catch(() => {}));
$("more-posts").addEventListener("click", () => loadPosts(state.postsPage.endCursor).catch(() => {}));
$("clear-parent").addEventListener("click", () => { $("parent-id").value = ""; });
$("listen").addEventListener("click", listen);
$("stop-listen").addEventListener("click", stopListening);

$("find-post-form").addEventListener("submit", (event) => {
  event.preventDefault();
  openPost($("find-post").value.trim()).catch(() => {});
});

$("find-comment-form").addEventListener("submit", (event) => {
  event.preventDefault();
  openBranch($("find-comment").value.trim()).catch(() => {});
});

loadPosts().catch(() => {});
