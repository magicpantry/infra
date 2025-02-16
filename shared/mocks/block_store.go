package mocks

import (
	"context"
	"errors"
	"io"
	"strings"

	"google.golang.org/api/iterator"

	"github.com/magicpantry/infra/shared/blockstore"
)

type BlockStore struct {
	buckets map[string]blockstore.Bucket

	ReturnErr bool
}

func (s *BlockStore) Bucket(v string) blockstore.Bucket {
	if s.buckets == nil {
		s.buckets = map[string]blockstore.Bucket{}
	}
	buk, ok := s.buckets[v]
	if !ok {
		buk = &Bucket{returnErr: s.ReturnErr}
		s.buckets[v] = buk
	}
	return buk
}

type Bucket struct {
	objects   map[string]blockstore.Object
	returnErr bool
}

func (b *Bucket) Add(v string) {
	if b.objects == nil {
		b.objects = map[string]blockstore.Object{}
	}
	b.objects[v] = &Object{
		Name:      v,
		returnErr: b.returnErr,
	}
}

func (b *Bucket) Object(v string) blockstore.Object {
	if b.objects == nil {
		b.objects = map[string]blockstore.Object{}
	}
	o, ok := b.objects[v]
	if !ok {
		o = &Object{
			Name:      v,
			returnErr: b.returnErr,
		}
		b.objects[v] = o
	}
	return o
}

func (b *Bucket) Objects(ctx context.Context, q *blockstore.Query) blockstore.Iterator {
	return &BlockStoreIterator{
		objects: b.objects,
		query:   q,
	}
}

type Object struct {
	Name        string
	DeleteCount int

	writer    *BlockstoreWriter
	returnErr bool
}

func (o *Object) NewReader(ctx context.Context) (io.ReadCloser, error) {
	return nil, nil
}

func (o *Object) NewWriter(ctx context.Context) blockstore.Writer {
	if o.writer == nil {
		o.writer = &BlockstoreWriter{returnErr: o.returnErr}
	}
	return o.writer
}

func (o *Object) Delete(ctx context.Context) error {
	if o.returnErr {
		return errors.New("error")
	}

	o.DeleteCount++
	return nil
}

type BlockstoreWriter struct {
	returnErr bool

	Content     []byte
	ContentType string
	CloseCount  int
}

func (w *BlockstoreWriter) Write(bs []byte) (n int, err error) {
	if w.returnErr {
		return -1, errors.New("error")
	}

	w.Content = bs
	return len(bs), nil
}

func (w *BlockstoreWriter) Close() error {
	w.CloseCount++
	return nil
}

func (w *BlockstoreWriter) SetContentType(t string) {
	w.ContentType = t
}

type BlockStoreIterator struct {
	objects map[string]blockstore.Object
	query   *blockstore.Query
	asList  []blockstore.Object
	index   int
	init    bool
}

func (i *BlockStoreIterator) Next() (*blockstore.Attrs, error) {
	if !i.init {
		for v, o := range i.objects {
			if !strings.HasPrefix(v, i.query.Prefix[:len(i.query.Prefix)-1]) {
				continue
			}
			i.asList = append(i.asList, o)
		}
	}
	i.init = true

	if i.index >= len(i.asList) {
		return nil, iterator.Done
	}

	o := i.asList[i.index]
	i.index++

	return &blockstore.Attrs{Name: o.(*Object).Name}, nil
}
