package mocks

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"google.golang.org/api/iterator"

	"github.com/magicpantry/infra/shared/docstore"
)

var mux sync.Mutex

type DocStore struct {
	collections map[string]docstore.Collection
	docs        map[string]docstore.Doc

	ReturnGetErr              bool
	ReturnSetErr              bool
	ReturnGetAllErr           bool
	ReturnRunTransactionErr   bool
	ReturnDeleteDocErr        bool
	ReturnCountErr            bool
	ReturnSnapshotIteratorErr bool
}

func (s *DocStore) Collection(name string) docstore.Collection {
	mux.Lock()
	defer mux.Unlock()

	if s.collections == nil {
		s.collections = map[string]docstore.Collection{}
	}
	if s.docs == nil {
		s.docs = map[string]docstore.Doc{}
	}
	c, ok := s.collections[name]

	if !ok {
		c = &Collection{
			collections:               s.collections,
			docs:                      s.docs,
			returnGetErr:              s.ReturnGetErr,
			returnSetErr:              s.ReturnSetErr,
			returnDeleteDocErr:        s.ReturnDeleteDocErr,
			returnCountErr:            s.ReturnCountErr,
			returnSnapshotIteratorErr: s.ReturnSnapshotIteratorErr,
		}
		s.collections[name] = c
	}
	return c
}

func (s *DocStore) GetAll(ctx context.Context, ds []docstore.Doc) ([]docstore.Snapshot, error) {
	if s.ReturnGetAllErr {
		return nil, errors.New("error")
	}

	var snaps []docstore.Snapshot
	for _, d := range ds {
		doc, ok := s.docs[d.ID()]
		if !ok {
			continue
		}
		snap, err := doc.Get(ctx)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, snap)
	}
	return snaps, nil
}

func (s *DocStore) RunTransaction(ctx context.Context, fn func(context.Context, docstore.Transaction) error) error {
	if s.ReturnRunTransactionErr {
		return errors.New("error")
	}

	return fn(ctx, &Transaction{docs: s.docs})
}

func (s *DocStore) BulkWriter(ctx context.Context) docstore.BulkWriter {
	return nil
}

type Transaction struct {
	docs map[string]docstore.Doc
}

func (t *Transaction) Delete(d docstore.Doc) error {
	return nil
}

func (t *Transaction) GetAll(ds []docstore.Doc) ([]docstore.Snapshot, error) {
	return nil, nil
}

func (t *Transaction) Get(d docstore.Doc) (docstore.Snapshot, error) {
	for _, doc := range t.docs {
		if doc.ID() != d.ID() {
			continue
		}
		return doc.Get(context.Background())
	}
	t.docs[d.ID()] = d
	return d.Get(context.Background())
}

func (t *Transaction) Set(d docstore.Doc, x map[string]interface{}) error {
	return d.Set(context.Background(), x)
}

type Collection struct {
	collections               map[string]docstore.Collection
	docs                      map[string]docstore.Doc
	returnGetErr              bool
	returnSetErr              bool
	returnDeleteDocErr        bool
	returnCountErr            bool
	returnSnapshotIteratorErr bool
}

func (c *Collection) Doc(name string) docstore.Doc {
	mux.Lock()
	defer mux.Unlock()

	d, ok := c.docs[name]
	if !ok {
		d = &Doc{
			id:                        name,
			collections:               c.collections,
			docs:                      c.docs,
			returnGetErr:              c.returnGetErr,
			returnSetErr:              c.returnSetErr,
			returnDeleteDocErr:        c.returnDeleteDocErr,
			returnCountErr:            c.returnCountErr,
			returnSnapshotIteratorErr: c.returnSnapshotIteratorErr,
		}
		c.docs[name] = d
	}
	return d
}

func (c *Collection) RandomDoc() docstore.Doc {
	return &Doc{}
}

func (c *Collection) Get(ctx context.Context) (docstore.Doc, error) {
	return &Doc{}, nil
}

func (c *Collection) GetAll(ctx context.Context) ([]docstore.Doc, error) {
	return nil, nil
}

func (c *Collection) Count(ctx context.Context) (int, error) {
	mux.Lock()
	defer mux.Unlock()

	if c.returnCountErr {
		return -1, errors.New("error")
	}

	return len(c.docs), nil
}

func (c *Collection) Query() docstore.Query {
	mux.Lock()
	defer mux.Unlock()

	return &Query{
		docs:                      c.docs,
		returnSnapshotIteratorErr: c.returnSnapshotIteratorErr,
	}
}

type Query struct {
	docs                      map[string]docstore.Doc
	keys                      []string
	ops                       []string
	values                    []any
	returnSnapshotIteratorErr bool
}

func (q *Query) Limit(n int) docstore.Query {
	return nil
}

func (q *Query) StartAfter(sn docstore.Snapshot) docstore.Query {
	return nil
}

func (q *Query) OrderBy(f string, so docstore.SortOrder) docstore.Query {
	return nil
}

func (q *Query) Where(key, op string, value any) docstore.Query {
	mux.Lock()
	defer mux.Unlock()

	q.keys = append(q.keys, key)
	q.ops = append(q.ops, op)
	q.values = append(q.values, value)
	return q
}

func (q *Query) Documents(ctx context.Context) docstore.SnapshotIterator {
	mux.Lock()
	defer mux.Unlock()

	n := len(q.keys)
	passes := []docstore.Doc{}
	for _, v := range q.docs {
		snap, _ := v.Get(ctx)
		ok := len(snap.Data()) > 0
		for i := 0; i < n; i++ {
			keyCasted, keyOk := snap.Data()[q.keys[i]].(string)
			valueCasted, valueOk := q.values[i].(string)
			if !keyOk || !valueOk {
				ok = false
				continue
			}
			if q.ops[i] == ">=" {
				if keyCasted < valueCasted {
					ok = false
				}
			}
			if q.ops[i] == "<" {
				if keyCasted >= valueCasted {
					ok = false
				}
			}
		}
		if !ok {
			continue
		}
		passes = append(passes, v)
	}
	sort.Slice(passes, func(a, b int) bool {
		return passes[a].ID() < passes[b].ID()
	})
	return &SnapshotIterator{
		docs:                      passes,
		returnSnapshotIteratorErr: q.returnSnapshotIteratorErr,
	}
}

type SnapshotIterator struct {
	docs                      []docstore.Doc
	index                     int
	returnSnapshotIteratorErr bool
}

func (i *SnapshotIterator) Next() (docstore.Snapshot, error) {
	mux.Lock()
	defer mux.Unlock()

	if i.returnSnapshotIteratorErr {
		return nil, errors.New("error")
	}

	if i.index >= len(i.docs) {
		return nil, iterator.Done
	}
	v := i.docs[i.index]
	i.index++
	return &Snapshot{doc: v}, nil
}

func (i *SnapshotIterator) Stop() {}

type Snapshot struct {
	doc docstore.Doc
}

func (s *Snapshot) Doc() docstore.Doc {
	mux.Lock()
	defer mux.Unlock()

	return s.doc
}

func (s *Snapshot) Data() map[string]interface{} {
	return s.doc.(*Doc).data
}

func (s *Snapshot) CreateTime() time.Time {
	return time.UnixMicro(0)
}

func (s *Snapshot) UpdateTime() time.Time {
	return time.UnixMicro(0)
}

func (s *Snapshot) ReadTime() time.Time {
	return time.UnixMicro(0)
}

type Doc struct {
	id                        string
	collections               map[string]docstore.Collection
	docs                      map[string]docstore.Doc
	data                      map[string]any
	returnDeleteDocErr        bool
	returnCountErr            bool
	returnSnapshotIteratorErr bool
	returnGetErr              bool
	returnSetErr              bool

	DeleteCount int
}

func (d *Doc) ID() string {
	return d.id
}

func (d *Doc) Collection(name string) docstore.Collection {
	mux.Lock()
	defer mux.Unlock()

	c, ok := d.collections[name]
	if !ok {
		c = &Collection{
			collections:               d.collections,
			docs:                      d.docs,
			returnGetErr:              d.returnGetErr,
			returnSetErr:              d.returnSetErr,
			returnDeleteDocErr:        d.returnDeleteDocErr,
			returnCountErr:            d.returnCountErr,
			returnSnapshotIteratorErr: d.returnSnapshotIteratorErr,
		}
		d.collections[name] = c
	}
	return c
}

func (d *Doc) Get(ctx context.Context) (docstore.Snapshot, error) {
	if d.returnGetErr {
		return nil, errors.New("error")
	}
	return &Snapshot{doc: d}, nil
}

func (d *Doc) Update(ctx context.Context, us []docstore.Update) error {
	return nil
}

func (d *Doc) Delete(ctx context.Context) error {
	mux.Lock()
	defer mux.Unlock()

	if d.returnDeleteDocErr {
		return errors.New("error")
	}

	d.DeleteCount++
	return nil
}

func (d *Doc) Set(ctx context.Context, x interface{}) error {
	mux.Lock()
	defer mux.Unlock()

	if d.returnSetErr {
		return errors.New("error")
	}

	d.data = x.(map[string]any)

	return nil
}

func (d *Doc) Merge(ctx context.Context, x interface{}) error {
	return nil
}
