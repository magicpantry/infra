package mocks

import (
	"context"

	"github.com/magicpantry/infra/shared/database"
)

type Database struct{}

func (d *Database) Single() database.Transaction {
	return nil
}

func (d *Database) Delete(ctx context.Context, table string, vs ...interface{}) error {
	return nil
}

func (d *Database) Upsert(ctx context.Context, table string, cs [][]database.Column) error {
	return nil
}

func (d *Database) Update(ctx context.Context, table string, cs [][]database.Column) error {
	return nil
}
