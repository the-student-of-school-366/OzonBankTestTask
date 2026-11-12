package api

import "OzonBankTestTask/internal/graphql"

// Алиасы сохраняют публичные типы API; потребляемые контракты и генерация моков
// находятся в internal/graphql/deps.go рядом с резолверами.
type PostUseCase = graphql.PostUseCase
type CommentUseCase = graphql.CommentUseCase
