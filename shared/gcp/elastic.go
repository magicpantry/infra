package gcp

import (
	"context"
	"fmt"
	"log"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	elastic "github.com/elastic/go-elasticsearch/v8"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"

	"github.com/magicpantry/infra/shared/config"
)

func ElasticClient(ctx context.Context, cfg config.Config) *elastic.Client {
	return ElasticClientWithParams(cfg.Strings(ctx, "ELASTIC_URLS"), "ELASTIC_USERNAME", "ELASTIC_PASSWORD")
}

func ElasticClientWithParams(urls []string, usernameEnvKey, passwordEnvKey string) *elastic.Client {
	username := os.Getenv(usernameEnvKey)
	password := os.Getenv(passwordEnvKey)

	if username == "" || password == "" {
		ctx := context.Background()
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()

		usernameReq := &secretmanagerpb.AccessSecretVersionRequest{
			Name: fmt.Sprintf("projects/magicpantryio/secrets/%s/versions/%s", usernameEnvKey, "1"),
		}
		passwordReq := &secretmanagerpb.AccessSecretVersionRequest{
			Name: fmt.Sprintf("projects/magicpantryio/secrets/%s/versions/%s", passwordEnvKey, "1"),
		}

		usernameSecret, err := client.AccessSecretVersion(ctx, usernameReq)
		if err != nil {
			log.Fatal(err)
		}
		passwordSecret, err := client.AccessSecretVersion(ctx, passwordReq)
		if err != nil {
			log.Fatal(err)
		}

		username = string(usernameSecret.Payload.Data)
		password = string(passwordSecret.Payload.Data)
	}

	es, err := elastic.NewClient(elastic.Config{
		Addresses: urls,
		Username:  username,
		Password:  password,
	})
	if err != nil {
		log.Fatalf("failed to create elastic client: %v", err)
	}

	return es
}
