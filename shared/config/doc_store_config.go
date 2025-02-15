package config

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/magicpantry/infra/shared/docstore"
)

type DocStoreConfig struct {
	Name     string
	DocStore docstore.DocStore
}

func (c *DocStoreConfig) Int(ctx context.Context, key string) int {
	if os.Getenv(key) != "" {
		v, err := strconv.Atoi(os.Getenv(key))
		if err != nil {
			log.Fatalf("failed to get '%v': %v'", key, err)
		}
		return v
	}
	snap, err := c.DocStore.Collection(c.Name).Doc(key).Get(ctx)
	if err != nil {
		log.Fatalf("failed to get '%v': %v'", key, err)
	}
	if snap.Data() == nil {
		log.Fatalf("'%v' doesn't exist in config", key)
	}

	v, ok := snap.Data()["value"].(int64)
	if !ok {
		log.Fatalf("'%v' isn't an int: %v", key, snap.Data()["value"])
	}

	return int(v)
}

func (c *DocStoreConfig) String(ctx context.Context, key string) string {
	if os.Getenv(key) != "" {
		return os.Getenv(key)
	}

	snap, err := c.DocStore.Collection(c.Name).Doc(key).Get(ctx)
	if err != nil {
		log.Fatalf("failed to get '%v': %v'", key, err)
	}
	if snap.Data() == nil {
		log.Fatalf("'%v' doesn't exist in config", key)
	}

	v, ok := snap.Data()["value"].(string)
	if !ok {
		log.Fatalf("'%v' isn't a string: %v", key, snap.Data()["value"])
	}

	return v
}

func (c *DocStoreConfig) Strings(ctx context.Context, key string) []string {
	snap, err := c.DocStore.Collection(c.Name).Doc(key).Get(ctx)
	if err != nil {
		log.Fatalf("failed to get '%v': %v'", key, err)
	}
	if snap.Data() == nil {
		log.Fatalf("'%v' doesn't exist in config", key)
	}

	xs, ok := snap.Data()["value"].([]interface{})
	if !ok {
		log.Fatalf("'%v' isn't a string list: %v", key, snap.Data()["value"])
	}

	var vs []string
	for _, x := range xs {
		v, ok := x.(string)
		if !ok {
			log.Fatalf("'%v' isn't a string in a string list: %v", key, x)
		}
		vs = append(vs, v)
	}

	return vs
}
