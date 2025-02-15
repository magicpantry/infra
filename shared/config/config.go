package config

import "context"

type Config interface {
	Int(context.Context, string) int
	String(context.Context, string) string
	Strings(context.Context, string) []string
}
