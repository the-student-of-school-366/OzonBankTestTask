INSERT INTO comments (id, post_id, parent_id, author_id, text, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, post_id, parent_id, author_id, text, created_at
