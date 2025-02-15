package gcp

import (
	"context"
	"log"

	"cloud.google.com/go/storage"
)

func StorageClient(ctx context.Context) *storage.Client {
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to connect to cloud storage: %v", err)
	}

	return client
}
