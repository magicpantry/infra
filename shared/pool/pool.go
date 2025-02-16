package pool

import (
	"context"

	"github.com/magicpantry/infra/shared/docstore"
)

type Pool interface {
	Add(context.Context, interface{}) error
}

type DocStorePool struct {
	DocStore docstore.DocStore

	PoolName string
	Limit    int
}

func (p *DocStorePool) Add(ctx context.Context, x interface{}) error {
	poolCollection := p.DocStore.Collection("insights").Doc("pools").Collection(p.PoolName)

	n, err := poolCollection.Count(ctx)
	if err != nil {
		return err
	}

	// Delete one entry to work towards making room for the data.
	if n >= p.Limit {
		doc, err := poolCollection.Get(ctx)
		if err != nil {
			return err
		}
		if doc == nil {
			return nil
		}

		if err := doc.Delete(ctx); err != nil {
			return err
		}

		return nil
	} else {
		return poolCollection.RandomDoc().Set(ctx, x)
	}
}
