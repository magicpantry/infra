package database

import (
	"context"

	"cloud.google.com/go/spanner"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/api/iterator"
)

type SpannerRow struct {
	row *spanner.Row
}

func (r *SpannerRow) ColumnByName(field string, x interface{}) error {
	return r.row.ColumnByName(field, x)
}

type SpannerIterator struct {
	iterator *spanner.RowIterator
}

func (i *SpannerIterator) Next() (Row, error) {
	row, err := i.iterator.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &SpannerRow{row: row}, nil
}

type SpannerTransaction struct {
	transaction *spanner.ReadOnlyTransaction
}

func (tx *SpannerTransaction) Query(ctx context.Context, stmt Statement) Iterator {
	return &SpannerIterator{
		iterator: tx.transaction.Query(ctx, spanner.Statement{
			SQL:    stmt.SQL,
			Params: stmt.Params,
		}),
	}
}

func (tx *SpannerTransaction) Close() {
	tx.transaction.Close()
}

type SpannerDatabase struct {
	Client    *spanner.Client
	BatchSize int
}

func (db *SpannerDatabase) Single() Transaction {
	return &SpannerTransaction{transaction: db.Client.Single()}
}

func (db *SpannerDatabase) apply(ctx context.Context, table string, css [][]Column, m mutator) error {
	batchSize := 1
	if db.BatchSize > 1 {
		batchSize = db.BatchSize
	}

	var multi error
	for i := 0; i < len(css); i += batchSize {
		distance := batchSize
		if i+distance > len(css)-i {
			distance = len(css) - i
		}

		var mutations []*spanner.Mutation
		for _, cs := range css[i : i+distance] {
			var columns []string
			var values []interface{}
			for _, c := range cs {
				columns = append(columns, c.Field)
				values = append(values, c.Value)
			}
			mutations = append(
				mutations,
				m(table, columns, values))
		}
		_, err := db.Client.Apply(ctx, mutations)
		if err != nil {
			multi = multierror.Append(multi, err)
		}
	}
	return multi
}

type mutator func(table string, cols []string, vals []interface{}) *spanner.Mutation

func (db *SpannerDatabase) Upsert(ctx context.Context, table string, css [][]Column) error {
	return db.apply(ctx, table, css, spanner.InsertOrUpdate)
}

func (db *SpannerDatabase) Update(ctx context.Context, table string, css [][]Column) error {
	return db.apply(ctx, table, css, spanner.Update)
}

func (db *SpannerDatabase) Delete(ctx context.Context, table string, keys ...interface{}) error {
	mutations := []*spanner.Mutation{spanner.Delete(table, spanner.Key(keys))}
	_, err := db.Client.Apply(ctx, mutations)
	return err
}
