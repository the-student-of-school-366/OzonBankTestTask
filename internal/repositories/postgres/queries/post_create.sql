INSERT INTO posts (id, title, content, author_id, comments_enabled, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, title, content, author_id, comments_enabled, created_at
