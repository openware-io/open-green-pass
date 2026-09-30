package domain

import "context"

type GenerateRequest struct {
	RequestID string
	Model     string
	Prompt    string
	MaxTokens int
}

type GenerateResult struct {
	RequestID string
	Model     string
	Text      string
	TokensIn  int
	TokensOut int
}

type ModelProviderPort interface {
	Name() string
	Generate(context.Context, GenerateRequest) (GenerateResult, error)
}

type MeterPort interface {
	Record(context.Context, GenerateResult) error
}
