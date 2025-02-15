package genai

import (
	"context"

	gcpgenai "cloud.google.com/go/vertexai/genai"
)

type Part gcpgenai.Part

type GenAI interface {
	SetSystemInstructions(instructions string)
	SetResponseMIMEType(mimeType string)

	GenerateContent(ctx context.Context, parts ...Part) (string, error)

	Close()
}
