package future

import (
	"fmt"
	"runtime/debug"

	"github.com/hashicorp/go-multierror"

	"github.com/magicpantry/infra/shared/box"
)

type value[T any] box.Pair[T, error]

type Result[T any] struct {
	waiter chan value[T]
}

func (r *Result[T]) Wait() (T, error) {
	v := <-r.waiter
	return v.A, v.B
}

type Op func() error

func (f Op) Start() *Result[any] {
	done := make(chan value[any])
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- value[any]{B: fmt.Errorf("panic: %v\n%v", r, string(debug.Stack()))}
			}
		}()

		err := f()
		done <- value[any]{
			A: struct{}{},
			B: err,
		}
	}()
	return &Result[any]{waiter: done}
}

func WaitForErr[T any](res *Result[T]) error {
	_, err := res.Wait()
	return err
}

type Fn[X, Y any] func(X) (Y, error)

func (f Fn[X, Y]) Start(x X) *Result[Y] {
	done := make(chan value[Y])
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- value[Y]{B: fmt.Errorf("panic: %v\n%v", r, string(debug.Stack()))}
			}
		}()

		y, err := f(x)
		done <- value[Y]{
			A: y,
			B: err,
		}
	}()
	return &Result[Y]{waiter: done}
}

func Combine(errs ...error) error {
	var multi error
	for _, err := range errs {
		if err == nil {
			continue
		}
		multi = multierror.Append(multi, err)
	}
	return multi
}
