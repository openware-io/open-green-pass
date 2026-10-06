package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/cicd/domain"
)

func hmacForTest(secret []byte, timestamp time.Time, payload []byte) string {
	m := hmac.New(sha256.New, secret)
	_, _ = fmt.Fprintf(m, "%d.", timestamp.Unix())
	_, _ = m.Write(payload)
	return hex.EncodeToString(m.Sum(nil))
}

func TestHMACVerifierContract(t *testing.T) {
	now := time.Unix(1700000000, 0)
	body := []byte(`{"delivery":"d1"}`)
	v := HMACSHA256Verifier{Secret: []byte("secret"), Skew: time.Minute}
	mac := hmacForTest([]byte("secret"), now, body)
	if err := v.Verify(body, "sha256="+mac, now, now); err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(body, "sha256="+mac, now.Add(-2*time.Minute), now); err != ErrExpiredSignature {
		t.Fatalf("err=%v", err)
	}
	if err := v.Verify([]byte("tampered"), "sha256="+mac, now, now); err != ErrInvalidSignature {
		t.Fatalf("err=%v", err)
	}
}

func TestMemoryOutboxRetryAndIdempotency(t *testing.T) {
	o := NewMemoryOutbox()
	now := time.Unix(0, 0)
	d := domain.OutboxDelivery{TeamID: 1, EventID: "e1", DeliveryKey: "k1", Provider: "fake", Status: domain.DeliveryPending}
	if err := o.Enqueue(d); err != nil {
		t.Fatal(err)
	}
	if err := o.Enqueue(d); err != domain.ErrDuplicateDelivery {
		t.Fatalf("duplicate=%v", err)
	}
	claimed, err := o.Claim(now)
	if err != nil || claimed == nil || claimed.Attempt != 1 {
		t.Fatalf("claim=%+v err=%v", claimed, err)
	}
	if err := o.MarkFailed("k1", "503", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if claimed, _ := o.Claim(now); claimed != nil {
		t.Fatal("claimed before retry time")
	}
}
