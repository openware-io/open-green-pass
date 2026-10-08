package infra

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/openware-io/open-green-pass/internal/cicd/domain"
)

// WebhookConnector delivers an outbox payload to an explicitly configured
// endpoint and uses the delivery key as the idempotency header.
type WebhookConnector struct {
	URL    string
	Client *http.Client
}

func (c WebhookConnector) Deliver(d domain.OutboxDelivery) error {
	if c.URL == "" {
		return fmt.Errorf("webhook URL is not configured")
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	body := d.Payload
	if len(body) == 0 {
		var err error
		body, err = json.Marshal(map[string]any{"event_id": d.EventID, "team_id": d.TeamID, "delivery_key": d.DeliveryKey})
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", d.DeliveryKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
