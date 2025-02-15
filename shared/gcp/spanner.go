package gcp

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/spanner"
)

func SpannerClient(ctx context.Context, db string) *spanner.Client {
	projectID := "magicpantryio"
	instanceID := "magicpantryio-db"
	client, err := spanner.NewClient(ctx, fmt.Sprintf(
		"projects/%s/instances/%s/databases/%s",
		projectID, instanceID, db))
	if err != nil {
		log.Fatalf("failed to create spanner client: %v", err)
	}

	return client
}
