package domain

import (
	"context"
	"errors"
)

var ErrRetryable = errors.New("ai: retryable provider failure")

// RetryableError marks failures for which a fallback provider may be tried
// (for example timeout, rate limit, or provider 5xx). Credential, request,
// policy and validation failures must not be wrapped with this marker.
type RetryableError struct{ Err error }

func (e RetryableError) Error() string        { return e.Err.Error() }
func (e RetryableError) Unwrap() error        { return e.Err }
func (e RetryableError) Is(target error) bool { return target == ErrRetryable }

func IsRetryable(err error) bool { return errors.Is(err, ErrRetryable) }

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
