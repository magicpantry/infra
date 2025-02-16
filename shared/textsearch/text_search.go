package textsearch

import (
	"context"
	"io"
)

type PaginationToken struct {
	Value string
}

type TextSearch[T, Filter, Sort any] interface {
	SearchPage(
		ctx context.Context,
		n int, pt *PaginationToken,
		f Filter, s Sort) ([]T, *PaginationToken, error)
	Index(ctx context.Context, id T, rd io.Reader) error
	Delete(ctx context.Context, id T) error
}
