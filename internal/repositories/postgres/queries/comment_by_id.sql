SELECT id, post_id, parent_id, author_id, text, created_at
FROM comments WHERE id = $1
