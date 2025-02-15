package gcp

import (
	"context"
	"errors"
	"log"

	gcpgenai "cloud.google.com/go/vertexai/genai"

	"github.com/magicpantry/infra/shared/genai"
)

type gcpGenAI struct {
	Client *gcpgenai.Client
	Model  *gcpgenai.GenerativeModel
}

func NewGenerativeAIClient(ctx context.Context, modelName string) genai.GenAI {
	client, err := gcpgenai.NewClient(ctx, "magicpantryio", "us-east1")
	if err != nil {
		log.Fatal(err)
	}

	model := client.GenerativeModel(modelName)

	return &gcpGenAI{
		Client: client,
		Model:  model,
	}
}

func (ai *gcpGenAI) SetSystemInstructions(instructions string) {
	ai.Model.SystemInstruction = &gcpgenai.Content{Parts: []gcpgenai.Part{gcpgenai.Text(instructions)}}
}

func (ai *gcpGenAI) SetResponseMIMEType(mimeType string) {
	ai.Model.ResponseMIMEType = mimeType
}

func (ai *gcpGenAI) GenerateContent(ctx context.Context, parts ...genai.Part) (string, error) {
	var casted []gcpgenai.Part
	for _, part := range parts {
		casted = append(casted, part)
	}
	resp, err := ai.Model.GenerateContent(ctx, casted...)
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 {
		return "", errors.New("no response")
	}
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(gcpgenai.Text); ok {
			return string(txt), nil
		}
	}

	return "", nil
}

func (ai *gcpGenAI) Close() {
	ai.Client.Close()
}
