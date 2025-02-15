package docstore

import (
	"context"

	"time"
)

type Transaction interface {
	Get(Doc) (Snapshot, error)
	Delete(Doc) error
	GetAll([]Doc) ([]Snapshot, error)
	Set(Doc, map[string]interface{}) error
}

type BulkWriter interface {
	Merge(Doc, interface{}) (Waiter, error)
	End()
}

type Waiter interface {
	Wait() error
}

type DocStore interface {
	Collection(name string) Collection
	GetAll(context.Context, []Doc) ([]Snapshot, error)
	RunTransaction(context.Context, func(context.Context, Transaction) error) error

	BulkWriter(context.Context) BulkWriter
}

type SnapshotIterator interface {
	Next() (Snapshot, error)
	Stop()
}

type Query interface {
	Limit(int) Query
	StartAfter(Snapshot) Query
	OrderBy(string, SortOrder) Query
	Where(key, op string, value any) Query
	Documents(context.Context) SnapshotIterator
}

type Collection interface {
	Doc(name string) Doc
	RandomDoc() Doc
	Get(ctx context.Context) (Doc, error)
	GetAll(ctx context.Context) ([]Doc, error)
	Count(ctx context.Context) (int, error)
	Query() Query
}

type Doc interface {
	ID() string
	Collection(name string) Collection
	Get(context.Context) (Snapshot, error)
	Update(context.Context, []Update) error
	Delete(context.Context) error
	Set(context.Context, interface{}) error
	Merge(context.Context, interface{}) error
}

type Snapshot interface {
	Doc() Doc
	CreateTime() time.Time
	UpdateTime() time.Time
	ReadTime() time.Time

	Data() map[string]interface{}
}

type Update struct {
	Path  string
	Value interface{}
}

type SortOrder int

const (
	Desc SortOrder = 1
	Asc  SortOrder = 2
)
