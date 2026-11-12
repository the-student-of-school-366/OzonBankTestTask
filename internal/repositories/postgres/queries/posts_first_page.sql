SELECT id, title, content, author_id, comments_enabled, created_at
FROM posts ORDER BY created_at DESC, id DESC LIMIT $1
