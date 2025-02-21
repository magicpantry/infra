package grpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"google.golang.org/grpc/metadata"
)

func ParseUID(ctx context.Context) (string, error) {
	key := "x-endpoint-api-userinfo"
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok || len(md.Get(key)) != 1 {
		return "", errors.New("couldn't parse metadata from context")
	}

	ut := md.Get(key)[0]
	bs, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(ut, "="))
	if err != nil {
		return "", err
	}

	var ui userInfo
	if err := json.Unmarshal(bs, &ui); err != nil {
		return "", err
	}

	return ui.UID, nil
}

type userInfo struct {
	UID string `json:"user_id"`
}
