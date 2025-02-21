package limit

import (
	"context"
	"errors"
	"time"

	"github.com/magicpantry/infra/shared/docstore"
)

type RateLimiter struct {
	DocStore docstore.DocStore

	Repo     string
	Bucket   string
	Count    int
	Duration time.Duration
}

func (rl *RateLimiter) Check(ctx context.Context, uid string) error {
	doc := rl.DocStore.Collection(rl.Repo).Doc("rate-limit").Collection(rl.Bucket).Doc(uid)
	now := time.Now().UTC()

	if err := rl.DocStore.RunTransaction(ctx, func(ctx context.Context, tx docstore.Transaction) error {
		snap, err := tx.Get(doc)
		if err != nil {
			return err
		}

		asData := snap.Data()
		if asData != nil {
			xs := asData["requests"].([]any)
			var reqs []time.Time
			for _, x := range xs {
				req := x.(time.Time)
				if now.Sub(req) > rl.Duration {
					continue
				}
				reqs = append(reqs, req)
			}

			if len(reqs) > rl.Count {
				return errors.New("rate limit")
			}

			reqs = append(reqs, now)

			if err := tx.Set(doc, map[string]any{
				"requests": reqs,
			}); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
