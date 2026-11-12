package graphql

//go:generate go -C ../.. tool gqlgen generate

type Resolver struct {
	posts    PostUseCase
	comments CommentUseCase
}
