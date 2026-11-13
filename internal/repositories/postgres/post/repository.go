package post

import (
	postusecase "OzonBankTestTask/internal/usecase/post"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ postusecase.Repository = (*Repository)(nil)
