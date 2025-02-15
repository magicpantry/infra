package docstore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirestoreWaiter struct {
	bulkWriterJob *firestore.BulkWriterJob
}

func (w *FirestoreWaiter) Wait() error {
	_, err := w.bulkWriterJob.Results()
	return err
}

type FirestoreBulkWriter struct {
	bulkWriter *firestore.BulkWriter
}

func (bw *FirestoreBulkWriter) Merge(doc Doc, v interface{}) (Waiter, error) {
	bwj, err := bw.bulkWriter.Set(doc.(*FirestoreDoc).doc, v, firestore.MergeAll)
	if err != nil {
		return nil, err
	}
	return &FirestoreWaiter{bulkWriterJob: bwj}, nil
}

func (bw *FirestoreBulkWriter) End() {
	bw.bulkWriter.End()
}

type FirestoreTransaction struct {
	transaction *firestore.Transaction
}

func (t *FirestoreTransaction) Delete(doc Doc) error {
	return t.transaction.Delete(doc.(*FirestoreDoc).doc)
}

func (t *FirestoreTransaction) Get(doc Doc) (Snapshot, error) {
	snap, err := t.transaction.Get(doc.(*FirestoreDoc).doc)
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	}
	if err != nil {
		return &FirestoreSnapshot{}, nil
	}
	return &FirestoreSnapshot{id: doc.ID(), snapshot: snap}, nil
}

func (t *FirestoreTransaction) GetAll(docs []Doc) ([]Snapshot, error) {
	var firestoreDocs []*firestore.DocumentRef
	for _, doc := range docs {
		firestoreDocs = append(firestoreDocs, doc.(*FirestoreDoc).doc)
	}

	firestoreSnaps, err := t.transaction.GetAll(firestoreDocs)
	if err != nil {
		return nil, err
	}

	var snaps []Snapshot
	for _, snap := range firestoreSnaps {
		snaps = append(snaps, &FirestoreSnapshot{
			id: snap.Ref.ID, snapshot: snap,
		})
	}

	return snaps, nil
}

func (t *FirestoreTransaction) Set(doc Doc, v map[string]interface{}) error {
	return t.transaction.Set(doc.(*FirestoreDoc).doc, v)
}

type FirestoreDocStore struct {
	Client *firestore.Client
}

func (ds *FirestoreDocStore) BulkWriter(ctx context.Context) BulkWriter {
	return &FirestoreBulkWriter{bulkWriter: ds.Client.BulkWriter(ctx)}
}

func (ds *FirestoreDocStore) Collection(name string) Collection {
	return &FirestoreCollection{collection: ds.Client.Collection(name)}
}

func (ds *FirestoreDocStore) GetAll(ctx context.Context, docs []Doc) ([]Snapshot, error) {
	var fdocs []*firestore.DocumentRef
	for _, doc := range docs {
		fdocs = append(fdocs, doc.(*FirestoreDoc).doc)
	}
	fsnaps, err := ds.Client.GetAll(ctx, fdocs)
	if err != nil {
		return nil, err
	}

	var snaps []Snapshot
	for _, fsnap := range fsnaps {
		snaps = append(snaps, &FirestoreSnapshot{id: fsnap.Ref.ID, snapshot: fsnap})
	}

	return snaps, nil
}

func (ds *FirestoreDocStore) RunTransaction(ctx context.Context, fn func(context.Context, Transaction) error) error {
	return ds.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return fn(ctx, &FirestoreTransaction{transaction: tx})
	})
}

type FirestoreCollection struct {
	collection *firestore.CollectionRef
}

func (c *FirestoreCollection) Doc(name string) Doc {
	return &FirestoreDoc{id: name, doc: c.collection.Doc(name)}
}

func (c *FirestoreCollection) RandomDoc() Doc {
	doc := c.collection.NewDoc()
	return &FirestoreDoc{id: doc.ID, doc: doc}
}

func (c *FirestoreCollection) Get(ctx context.Context) (Doc, error) {
	fdocs, err := c.collection.Limit(1).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	if len(fdocs) != 1 {
		return nil, nil
	}
	return &FirestoreDoc{doc: fdocs[0].Ref}, nil
}

func (c *FirestoreCollection) GetAll(ctx context.Context) ([]Doc, error) {
	fdocs, err := c.collection.DocumentRefs(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	var docs []Doc
	for _, fdoc := range fdocs {
		docs = append(docs, &FirestoreDoc{
			id:  fdoc.ID,
			doc: fdoc,
		})
	}

	return docs, nil
}

func (c *FirestoreCollection) Count(ctx context.Context) (int, error) {
	ar, err := c.collection.NewAggregationQuery().WithCount("n").Get(ctx)
	if err != nil {
		return -1, err
	}

	v := ar["n"].(*firestorepb.Value)

	return int(v.GetIntegerValue()), nil
}

func (c *FirestoreCollection) Query() Query {
	return &FirestoreQuery{query: c.collection.Query}
}

type FirestoreQuery struct {
	query firestore.Query
}

func (q *FirestoreQuery) Where(key, op string, value any) Query {
	return &FirestoreQuery{query: q.query.Where(key, op, value)}
}

func (q *FirestoreQuery) StartAfter(s Snapshot) Query {
	fs := s.(*FirestoreSnapshot).snapshot
	return &FirestoreQuery{query: q.query.StartAfter(fs)}
}

func (q *FirestoreQuery) Limit(n int) Query {
	return &FirestoreQuery{query: q.query.Limit(n)}
}

func (q *FirestoreQuery) OrderBy(f string, so SortOrder) Query {
	if so == Desc {
		return &FirestoreQuery{query: q.query.OrderBy(f, firestore.Desc)}
	}
	return &FirestoreQuery{query: q.query.OrderBy(f, firestore.Asc)}
}

func (q *FirestoreQuery) Documents(ctx context.Context) SnapshotIterator {
	return &FirestoreSnapshotIterator{iterator: q.query.Documents(ctx)}
}

type FirestoreSnapshotIterator struct {
	iterator *firestore.DocumentIterator
}

func (i *FirestoreSnapshotIterator) Next() (Snapshot, error) {
	snap, err := i.iterator.Next()
	if err != nil {
		return nil, err
	}
	return &FirestoreSnapshot{id: snap.Ref.ID, snapshot: snap}, nil
}

func (i *FirestoreSnapshotIterator) Stop() {
	i.iterator.Stop()
}

type FirestoreDoc struct {
	id  string
	doc *firestore.DocumentRef
}

func (d *FirestoreDoc) Collection(name string) Collection {
	return &FirestoreCollection{collection: d.doc.Collection(name)}
}

func (d *FirestoreDoc) ID() string {
	return d.id
}

func (d *FirestoreDoc) Get(ctx context.Context) (Snapshot, error) {
	s, err := d.doc.Get(ctx)
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	}
	if err != nil {
		return &FirestoreSnapshot{}, nil
	}
	return &FirestoreSnapshot{id: d.id, snapshot: s}, nil
}

func (d *FirestoreDoc) Update(ctx context.Context, us []Update) error {
	var fus []firestore.Update
	for _, u := range us {
		fus = append(fus, firestore.Update{
			Path:  u.Path,
			Value: u.Value,
		})
	}
	_, err := d.doc.Update(ctx, fus)
	return err
}

func (d *FirestoreDoc) Delete(ctx context.Context) error {
	_, err := d.doc.Delete(ctx)
	return err
}

func (d *FirestoreDoc) Set(ctx context.Context, v interface{}) error {
	_, err := d.doc.Set(ctx, v)
	return err
}

func (d *FirestoreDoc) Merge(ctx context.Context, v interface{}) error {
	_, err := d.doc.Set(ctx, v, firestore.MergeAll)
	return err
}

type FirestoreSnapshot struct {
	id       string
	snapshot *firestore.DocumentSnapshot
}

func (s *FirestoreSnapshot) Doc() Doc {
	if s.snapshot == nil {
		return nil
	}
	return &FirestoreDoc{id: s.id, doc: s.snapshot.Ref}
}

func (s *FirestoreSnapshot) CreateTime() time.Time {
	if s.snapshot == nil {
		return time.Time{}
	}
	return s.snapshot.CreateTime
}

func (s *FirestoreSnapshot) UpdateTime() time.Time {
	if s.snapshot == nil {
		return time.Time{}
	}
	return s.snapshot.UpdateTime
}

func (s *FirestoreSnapshot) ReadTime() time.Time {
	if s.snapshot == nil {
		return time.Time{}
	}
	return s.snapshot.ReadTime
}

func (s *FirestoreSnapshot) Data() map[string]interface{} {
	if s.snapshot == nil {
		return nil
	}
	return s.snapshot.Data()
}
