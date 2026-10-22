// DSH gateway HTTP client: internal-only, bearer-token protected.
package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/domain"
)

// DSHGatewayClient calls the internal dsh-gateway Service.
type DSHGatewayClient struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewDSHGatewayClient creates the client; baseURL must not include a trailing slash.
func NewDSHGatewayClient(baseURL, token string) *DSHGatewayClient {
	return &DSHGatewayClient{
		baseURL: strings.TrimSuffix(baseURL, "/"), token: token,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

type gatewayJob struct {
	ID     string `json:"job_id"`
	State  string `json:"state"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

func (c *DSHGatewayClient) Submit(ctx context.Context, jobID, prompt string, timeoutMS int64, idempotencyKey string) (domain.DSHJob, error) {
	body := map[string]any{
		"job_id": jobID, "prompt": prompt, "timeout_ms": timeoutMS, "idempotency_key": idempotencyKey,
	}
	var out gatewayJob
	if err := c.do(ctx, http.MethodPost, "/internal/dsh/jobs", body, &out); err != nil {
		return domain.DSHJob{}, err
	}
	return domain.DSHJob{ID: out.ID, State: out.State, Output: out.Output, Error: out.Error}, nil
}

func (c *DSHGatewayClient) Get(ctx context.Context, jobID string) (domain.DSHJob, error) {
	var out gatewayJob
	if err := c.do(ctx, http.MethodGet, "/internal/dsh/jobs/"+jobID, nil, &out); err != nil {
		return domain.DSHJob{}, err
	}
	return domain.DSHJob{ID: out.ID, State: out.State, Output: out.Output, Error: out.Error}, nil
}

func (c *DSHGatewayClient) Cancel(ctx context.Context, jobID string) error {
	return c.do(ctx, http.MethodDelete, "/internal/dsh/jobs/"+jobID, nil, nil)
}

func (c *DSHGatewayClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return fmt.Errorf("dsh gateway %s %s: %s (%s)", method, path, res.Status, strings.TrimSpace(string(payload)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}
