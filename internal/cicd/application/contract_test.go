package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

type connectorStub struct {
	err   error
	calls int
}

func (c *connectorStub) Deliver(domain.OutboxDelivery) error { c.calls++; return c.err }

func TestDispatcherDeliversAndMarksSuccess(t *testing.T) {
	o := NewMemoryOutbox()
	if err := o.Enqueue(domain.OutboxDelivery{TeamID: 1, EventID: "e1", DeliveryKey: "k1", Provider: "fake", Status: domain.DeliveryPending}); err != nil {
		t.Fatal(err)
	}
	c := &connectorStub{}
	d := &Dispatcher{Outbox: o, Connectors: map[string]Connector{"fake": c}, Retry: RetryPolicy{MaxAttempts: 3}, Now: func() time.Time { return time.Unix(10, 0) }}
	claimed, err := d.DispatchOne()
	if err != nil || !claimed || c.calls != 1 {
		t.Fatalf("claimed=%v calls=%d err=%v", claimed, c.calls, err)
	}
	if next, _ := o.Claim(time.Unix(20, 0)); next != nil {
		t.Fatalf("delivered item was claimable: %+v", next)
	}
}

func TestDispatcherRetriesAndEventuallyFails(t *testing.T) {
	o := NewMemoryOutbox()
	if err := o.Enqueue(domain.OutboxDelivery{TeamID: 1, EventID: "e1", DeliveryKey: "k1", Provider: "fake", Status: domain.DeliveryPending}); err != nil {
		t.Fatal(err)
	}
	c := &connectorStub{err: errors.New("provider unavailable")}
	d := &Dispatcher{Outbox: o, Connectors: map[string]Connector{"fake": c}, Retry: RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second}, Now: func() time.Time { return time.Unix(10, 0) }}
	if _, err := d.DispatchOne(); err != nil {
		t.Fatal(err)
	}
	if next, _ := o.Claim(time.Unix(20, 0)); next != nil {
		t.Fatalf("terminal failed item was claimable: %+v", next)
	}
}
