package database

import "context"

type Statement struct {
	SQL    string
	Params map[string]interface{}
}

type Row interface {
	ColumnByName(string, interface{}) error
}

type Iterator interface {
	Next() (Row, error)
}

type Transaction interface {
	Query(context.Context, Statement) Iterator
	Close()
}

type Column struct {
	Field string
	Value interface{}
}

type Database interface {
	Single() Transaction
	Delete(context.Context, string, ...interface{}) error
	Upsert(context.Context, string, [][]Column) error
	Update(context.Context, string, [][]Column) error
}
