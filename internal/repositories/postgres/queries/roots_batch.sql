WITH requests AS (
    SELECT * FROM unnest($1::uuid[], $2::int4[], $3::timestamptz[], $4::uuid[])
    WITH ORDINALITY AS r(owner_id, first, after_time, after_id, ordinal)
)
SELECT r.ordinal, p.id IS NOT NULL,
       c.id, c.post_id, c.parent_id, c.author_id, c.text, c.created_at
FROM requests r
LEFT JOIN posts p ON p.id = r.owner_id
LEFT JOIN LATERAL (
    (SELECT id, post_id, parent_id, author_id, text, created_at FROM comments
     WHERE r.after_id IS NULL AND post_id = r.owner_id AND parent_id IS NULL
     ORDER BY created_at DESC, id DESC LIMIT r.first + 1)
    UNION ALL
    (SELECT id, post_id, parent_id, author_id, text, created_at FROM comments
     WHERE r.after_id IS NOT NULL AND post_id = r.owner_id AND parent_id IS NULL
       AND (created_at, id) < (r.after_time, r.after_id)
     ORDER BY created_at DESC, id DESC LIMIT r.first + 1)
) c ON p.id IS NOT NULL
ORDER BY r.ordinal, c.created_at DESC, c.id DESC
