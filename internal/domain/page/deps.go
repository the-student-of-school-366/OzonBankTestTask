package page

//go:generate go tool mockery --case=snake --all

type Item interface {
	PagePosition() Position
}
