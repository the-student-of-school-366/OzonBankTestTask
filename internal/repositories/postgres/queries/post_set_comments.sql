UPDATE posts SET comments_enabled = $2 WHERE id = $1
RETURNING id, title, content, author_id, comments_enabled, created_at
