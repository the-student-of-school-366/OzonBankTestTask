package comment_test

import (
	"strings"
	"testing"

	"OzonBankTestTask/internal/domain"
	"OzonBankTestTask/internal/dto"
	"OzonBankTestTask/internal/repositories/postgres"
	"OzonBankTestTask/internal/repositories/postgres/queries"
	commentusecase "OzonBankTestTask/internal/usecase/comment"
	"OzonBankTestTask/internal/usecase/comment/mocks"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestCreatePlacementAndIndependentCopies(t *testing.T) {
	_, posts, comments := repositories(t)
	a, b := newPost(t, posts), newPost(t, posts)
	root, err := comments.CreateComment(t.Context(), newComment(t, a.ID, nil, strings.Repeat("я", 2000)))
	require.NoError(t, err)
	require.Nil(t, root.ParentID)
	reply, err := comments.CreateComment(t.Context(), newComment(t, a.ID, &root.ID, "reply"))
	require.NoError(t, err)
	require.Equal(t, root.ID, *reply.ParentID)
	*reply.ParentID = uuid.Nil
	values, err := comments.CommentsByID(t.Context(), []uuid.UUID{root.ID, reply.ID, uuid.New(), reply.ID})
	require.NoError(t, err)
	require.Len(t, values, 2)
	require.Equal(t, root.ID, *values[reply.ID].ParentID)
	_, err = comments.CreateComment(t.Context(), newComment(t, b.ID, &root.ID, "wrong post"))
	require.ErrorIs(t, err, domain.ErrParentPostMismatch)
	missing := uuid.New()
	_, err = comments.CreateComment(t.Context(), newComment(t, a.ID, &missing, "missing parent"))
	require.ErrorIs(t, err, domain.ErrCommentNotFound)
	_, err = comments.CreateComment(t.Context(), newComment(t, missing, nil, "missing post"))
	require.ErrorIs(t, err, domain.ErrPostNotFound)
	_, err = posts.SetCommentsEnabled(t.Context(), a.ID, "owner", false)
	require.NoError(t, err)
	_, err = comments.CreateComment(t.Context(), newComment(t, a.ID, nil, "disabled"))
	require.ErrorIs(t, err, domain.ErrCommentsDisabled)
}

func TestDatabaseConstraintsProtectDirectWrites(t *testing.T) {
	pool, posts, comments := repositories(t)
	a, b := newPost(t, posts), newPost(t, posts)
	root, err := comments.CreateComment(t.Context(), newComment(t, a.ID, nil, "root"))
	require.NoError(t, err)
	for _, tc := range []struct {
		name, text, code string
		parent           *uuid.UUID
		postID           uuid.UUID
	}{
		{name: "other post parent", postID: b.ID, parent: &root.ID, text: "reply", code: "23503"},
		{name: "too many runes", postID: a.ID, text: strings.Repeat("я", 2001), code: "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(t.Context(), queries.CommentCreate, postgres.UUID(uuid.New()), postgres.UUID(tc.postID), postgres.NullableUUID(tc.parent), "author", tc.text, a.CreatedAt)
			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr)
			require.Equal(t, tc.code, pgErr.Code)
		})
	}
}

func TestCommitFailureRollsBackAndDoesNotPublish(t *testing.T) {
	pool, posts, comments := repositories(t)
	post := newPost(t, posts)
	_, err := pool.Exec(t.Context(), `CREATE FUNCTION reject_comment_commit() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN RAISE EXCEPTION 'test commit failure'; END $$;
	CREATE CONSTRAINT TRIGGER reject_commit AFTER INSERT ON comments DEFERRABLE INITIALLY DEFERRED
	FOR EACH ROW EXECUTE FUNCTION reject_comment_commit();`)
	require.NoError(t, err)
	publisher := mocks.NewPublisher(t)
	_, err = commentusecase.New(comments, publisher).Create(t.Context(), "reader", dto.CreateCommentInput{PostID: post.ID.String(), Text: "rollback"})
	require.ErrorIs(t, err, domain.ErrStorage)
	var count int
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT count(*) FROM comments").Scan(&count))
	require.Zero(t, count)
	require.Zero(t, pool.Stat().AcquiredConns())
}
