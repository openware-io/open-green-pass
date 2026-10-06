package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidSignature = errors.New("cicd: invalid signature")
	ErrExpiredSignature = errors.New("cicd: expired signature")
)

// HMACSHA256Verifier verifies signatures in the provider-neutral format
// "sha256=<hex digest>" over "<unix timestamp>.<raw body>".
// Provider adapters may translate their native headers into this format.
type HMACSHA256Verifier struct {
	Secret []byte
	Skew   time.Duration
}

func (v HMACSHA256Verifier) Verify(payload []byte, signature string, timestamp, now time.Time) error {
	if len(v.Secret) == 0 || timestamp.IsZero() || strings.TrimSpace(signature) == "" {
		return ErrInvalidSignature
	}
	skew := v.Skew
	if skew <= 0 {
		skew = 5 * time.Minute
	}
	if timestamp.Before(now.Add(-skew)) || timestamp.After(now.Add(skew)) {
		return ErrExpiredSignature
	}
	mac := hmac.New(sha256.New, v.Secret)
	_, _ = fmt.Fprintf(mac, "%d.", timestamp.Unix())
	_, _ = mac.Write(payload)
	want := mac.Sum(nil)
	value := strings.TrimSpace(signature)
	if !strings.HasPrefix(value, "sha256=") {
		return ErrInvalidSignature
	}
	got, err := hex.DecodeString(strings.TrimPrefix(value, "sha256="))
	if err != nil || !hmac.Equal(got, want) {
		return ErrInvalidSignature
	}
	return nil
}

// ParseUnixTimestamp is a small adapter helper for provider headers.
func ParseUnixTimestamp(value string) (time.Time, error) {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(seconds, 0), nil
}
