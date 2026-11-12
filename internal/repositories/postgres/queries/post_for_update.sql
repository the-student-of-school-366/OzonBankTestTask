SELECT id, title, content, author_id, comments_enabled, created_at
FROM posts WHERE id = $1 FOR UPDATE
