package gcp

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
)

func FirestoreClient(ctx context.Context) *firestore.Client {
	projectID := "magicpantryio"

	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("failed to create firestore client: %v", err)
	}

	return client
}

func FirestoreClientWithDatabase(ctx context.Context, db string) *firestore.Client {
	projectID := "magicpantryio"

	client, err := firestore.NewClientWithDatabase(ctx, projectID, db)
	if err != nil {
		log.Fatalf("failed to create firestore client: %v", err)
	}

	return client
}
