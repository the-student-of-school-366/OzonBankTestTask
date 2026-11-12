SELECT id, title, content, author_id, comments_enabled, created_at
FROM posts WHERE (created_at, id) < ($1::timestamptz, $2::uuid)
ORDER BY created_at DESC, id DESC LIMIT $3
