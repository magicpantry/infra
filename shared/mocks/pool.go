package mocks

import (
	"context"
	"sync"
	"time"
)

type Pool struct {
	count int
	mux   sync.Mutex

	AddCalls []interface{}
}

func (p *Pool) WaitForCount(i int) {
	done := false
	for !done {
		func() {
			p.mux.Lock()
			defer p.mux.Unlock()

			if len(p.AddCalls) >= i {
				done = true
			}
		}()

		if !done {
			time.Sleep(250 * time.Millisecond)
		}
	}
}

func (p *Pool) Add(ctx context.Context, x interface{}) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	p.AddCalls = append(p.AddCalls, x)
	return nil
}
