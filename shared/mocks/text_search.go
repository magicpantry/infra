package mocks

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/magicpantry/infra/shared/textsearch"
)

type SearchPageCall[Filter, Sort comparable] struct {
	N               int
	PaginationToken *textsearch.PaginationToken
	Filter          Filter
	Sort            Sort
}

type IndexCall[T comparable] struct {
	ID    T
	Bytes []byte
}

type SearchPageResult[T comparable] struct {
	Ts              []T
	PaginationToken *textsearch.PaginationToken
	Err             error
}

type TextSearch[T, Filter, Sort comparable] struct {
	deleteCount map[T]int
	index       int

	SearchPageCalls   []SearchPageCall[Filter, Sort]
	SearchPageResults []SearchPageResult[T]
	IndexCalls        []IndexCall[T]
	ReturnErr         bool
}

func (ts *TextSearch[T, Filter, Sort]) SearchPage(
	ctx context.Context,
	n int, pt *textsearch.PaginationToken,
	f Filter, s Sort) ([]T, *textsearch.PaginationToken, error) {
	if ts.ReturnErr {
		return nil, nil, errors.New("error")
	}

	ts.SearchPageCalls = append(ts.SearchPageCalls, SearchPageCall[Filter, Sort]{
		N:               n,
		PaginationToken: pt,
		Filter:          f,
		Sort:            s,
	})
	if ts.index >= len(ts.SearchPageResults) {
		return nil, nil, nil
	}
	result := ts.SearchPageResults[ts.index]
	ts.index++
	return result.Ts, result.PaginationToken, result.Err
}

func (ts *TextSearch[T, Filter, Sort]) Index(ctx context.Context, id T, rd io.Reader) error {
	if ts.ReturnErr {
		return errors.New("error")
	}

	bs, err := io.ReadAll(rd)
	if err != nil {
		return err
	}

	ts.IndexCalls = append(ts.IndexCalls, IndexCall[T]{
		ID:    id,
		Bytes: bs,
	})

	return nil
}

func (ts *TextSearch[T, Filter, Sort]) Delete(ctx context.Context, id T) error {
	if ts.ReturnErr {
		return errors.New("error")
	}

	if ts.deleteCount == nil {
		ts.deleteCount = map[T]int{}
	}
	if _, ok := ts.deleteCount[id]; !ok {
		ts.deleteCount[id] = 0
	}
	ts.deleteCount[id]++
	return nil
}

func (ts *TextSearch[T, Filter, Sort]) DeleteCount(id T) int {
	for v, c := range ts.deleteCount {
		if fmt.Sprintf("%v", v) != fmt.Sprintf("%v", id) {
			continue
		}
		return c
	}
	return 0
}
