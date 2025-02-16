package mocks

import (
	"context"
	"errors"
)

type ClientFactory[T any] struct {
	Factory   func(string) T
	ReturnErr bool

	Clients map[string]T
}

func (f *ClientFactory[T]) Func() func(context.Context, string) (T, error) {

	return func(ctx context.Context, host string) (T, error) {
		var t T
		if f.ReturnErr {
			return t, errors.New("error")
		}
		if f.Clients == nil {
			f.Clients = map[string]T{}
		}
		t, ok := f.Clients[host]
		if !ok {
			t = f.Factory(host)
			f.Clients[host] = t
		}
		return t, nil
	}
}
