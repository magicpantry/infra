package grpc

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var tokenFile = flag.String("token-file", "", "token file")

func TokenSource(ctx context.Context, host string) oauth2.TokenSource {
	ts, err := TokenSourceErr(ctx, host)
	if err != nil {
		log.Fatal(err)
	}
	return ts
}

func TokenSourceErr(ctx context.Context, host string) (oauth2.TokenSource, error) {
	clientOptions := []idtoken.ClientOption{}
	if *tokenFile != "" {
		clientOptions = append(clientOptions, idtoken.WithCredentialsFile(*tokenFile))
	}

	ts, err := idtoken.NewTokenSource(ctx, "https://"+host, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to make token source: %v", err)
	}

	t, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to make token: %v", err)
	}

	return oauth2.ReuseTokenSource(t, ts), nil
}

type Contextualizer interface {
	Contextualize(context.Context) context.Context
}

func ContextualizerForHost(ctx context.Context, host string) Contextualizer {
	if strings.Contains(host, ":") {
		return &stubContextualizer{}
	}

	return &authContextualizer{TokenSource: TokenSource(ctx, host)}
}

type stubContextualizer struct{}

func (c *stubContextualizer) Contextualize(ctx context.Context) context.Context {
	return ctx
}

type authContextualizer struct {
	TokenSource oauth2.TokenSource
}

func (c *authContextualizer) Contextualize(ctx context.Context) context.Context {
	t, err := c.TokenSource.Token()
	if err != nil {
		ctx, cf := context.WithCancel(ctx)
		cf()
		return ctx
	}

	local := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+t.AccessToken)
	return local
}

func ContextualizedRequest[X, Y any](c Contextualizer, ctx context.Context, fn func(context.Context, X, ...grpc.CallOption) (Y, error)) func(X) (Y, error) {
	return func(x X) (Y, error) {
		return fn(c.Contextualize(ctx), x)
	}
}
