package blockstore

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
)

type StorageIterator struct {
	Iterator *storage.ObjectIterator
}

func (i *StorageIterator) Next() (*Attrs, error) {
	attrs, err := i.Iterator.Next()
	if err != nil {
		return nil, err
	}

	return &Attrs{Name: attrs.Name}, nil
}

type StorageBlockStore struct {
	Client *storage.Client
}

func (bs *StorageBlockStore) Bucket(bn string) Bucket {
	return &StorageBucket{Bucket: bs.Client.Bucket(bn)}
}

type StorageBucket struct {
	Bucket *storage.BucketHandle
}

func (b *StorageBucket) Object(on string) Object {
	return &StorageObject{Object: b.Bucket.Object(on)}
}

func (b *StorageBucket) Objects(ctx context.Context, q *Query) Iterator {
	return &StorageIterator{
		Iterator: b.Bucket.Objects(ctx, &storage.Query{Prefix: q.Prefix}),
	}
}

type StorageObject struct {
	Object *storage.ObjectHandle
}

func (o *StorageObject) NewReader(ctx context.Context) (io.ReadCloser, error) {
	return o.Object.NewReader(ctx)
}

func (o *StorageObject) NewWriter(ctx context.Context) Writer {
	return &StorageObjectWriter{Writer: o.Object.NewWriter(ctx)}
}

func (o *StorageObject) Delete(ctx context.Context) error {
	return o.Object.Delete(ctx)
}

type StorageObjectWriter struct {
	Writer *storage.Writer
}

func (s *StorageObjectWriter) Write(bs []byte) (int, error) {
	return s.Writer.Write(bs)
}

func (s *StorageObjectWriter) SetContentType(ct string) {
	s.Writer.ContentType = ct
}

func (s *StorageObjectWriter) Close() error {
	return s.Writer.Close()
}
