package blockstore

import (
	"context"
	"io"
)

type Attrs struct {
	Name string
}

type Iterator interface {
	Next() (*Attrs, error)
}

type Query struct {
	Prefix string
}

type Writer interface {
	io.WriteCloser
	SetContentType(string)
}

type Object interface {
	NewReader(context.Context) (io.ReadCloser, error)
	NewWriter(context.Context) Writer
	Delete(context.Context) error
}

type Bucket interface {
	Object(string) Object
	Objects(context.Context, *Query) Iterator
}

type BlockStore interface {
	Bucket(string) Bucket
}
