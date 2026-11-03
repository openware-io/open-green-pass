package infra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

func newTestRunEventNotifier() *RunEventNotifier {
	notifier := NewRunEventNotifier(nil, nil)
	notifier.ready.Store(true)
	return notifier
}

func TestRunEventNotifierDispatchesToRunSubscribersOnly(t *testing.T) {
	notifier := newTestRunEventNotifier()
	first, cancelFirst, err := notifier.Subscribe(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelFirst()
	second, cancelSecond, err := notifier.Subscribe(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelSecond()
	other, cancelOther, err := notifier.Subscribe(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelOther()

	notifier.dispatch(100)
	for name, channel := range map[string]<-chan struct{}{"first": first, "second": second} {
		select {
		case <-channel:
		case <-time.After(time.Second):
			t.Fatalf("%s subscriber was not woken", name)
		}
	}
	select {
	case <-other:
		t.Fatal("subscriber of another run was woken")
	default:
	}

	// Hints coalesce: a burst must not block and one pending hint is enough.
	notifier.dispatch(100)
	notifier.dispatch(100)
	select {
	case <-first:
	default:
		t.Fatal("coalesced hint was lost")
	}

	cancelFirst()
	notifier.dispatch(100)
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("remaining subscriber was not woken")
	}
	select {
	case <-first:
		t.Fatal("cancelled subscriber was still woken")
	default:
	}
}

func TestRunEventNotifierSubscribeRequiresReadyListener(t *testing.T) {
	notifier := NewRunEventNotifier(nil, nil)
	if notifier.Ready() {
		t.Fatal("notifier reported ready without a listener")
	}
	if _, _, err := notifier.Subscribe(context.Background(), 1); !errors.Is(err, domain.ErrGenerationRunEventNotifierUnavailable) {
		t.Fatalf("subscribe error = %v, want unavailable", err)
	}
	// A nil notifier must not panic in SSE composition.
	var nilNotifier *RunEventNotifier
	if _, _, err := nilNotifier.Subscribe(context.Background(), 1); !errors.Is(err, domain.ErrGenerationRunEventNotifierUnavailable) {
		t.Fatalf("nil notifier subscribe error = %v", err)
	}
}
