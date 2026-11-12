-- +goose Up
CREATE TABLE posts (
    id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    title text NOT NULL CHECK (char_length(title) > 0),
    content text NOT NULL CHECK (char_length(content) > 0),
    author_id text NOT NULL CHECK (char_length(author_id) > 0),
    comments_enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz(6) NOT NULL
);

CREATE TABLE comments (
    id uuid PRIMARY KEY CHECK (id <> '00000000-0000-0000-0000-000000000000'),
    post_id uuid NOT NULL,
    parent_id uuid,
    author_id text NOT NULL CHECK (char_length(author_id) > 0),
    text text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 2000),
    created_at timestamptz(6) NOT NULL,
    CONSTRAINT comments_post_fk FOREIGN KEY (post_id) REFERENCES posts(id),
    CONSTRAINT comments_post_id_unique UNIQUE (post_id, id),
    CONSTRAINT comments_parent_fk FOREIGN KEY (post_id, parent_id) REFERENCES comments(post_id, id),
    CONSTRAINT comments_not_self CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE INDEX posts_page_idx ON posts (created_at DESC, id DESC);
CREATE INDEX comments_roots_page_idx ON comments (post_id, created_at DESC, id DESC) WHERE parent_id IS NULL;
CREATE INDEX comments_replies_page_idx ON comments (parent_id, created_at DESC, id DESC) WHERE parent_id IS NOT NULL;

-- +goose Down
DROP TABLE comments;
DROP TABLE posts;
