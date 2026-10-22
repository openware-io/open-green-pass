package dshgw

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// SDKRunner runs one DSH SDK process per job over newline-delimited JSON-RPC.
// The process and its workspace are disposable; cancellation kills the process.
type SDKRunner struct {
	Binary    string
	Profile   string
	Provider  string
	Model     string
	APIKey    string
	MaxTokens int
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Run starts a fresh DSH SDK process, initializes the route, prompts, waits for
// idle, then shuts down. The process is always killed on return.
func (r SDKRunner) Run(ctx context.Context, job Job) (Result, error) {
	binary := r.Binary
	if binary == "" {
		binary = "dsh"
	}
	profile := r.Profile
	if profile == "" {
		profile = "sdk"
	}
	workspace, err := os.MkdirTemp("", "gp-dsh-job-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(workspace)
	home, err := os.MkdirTemp("", "gp-dsh-home-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(home)

	cmd := exec.Command(binary, "--profile", profile)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), "DSH_HOME="+home)
	if r.APIKey != "" {
		cmd.Env = append(cmd.Env, "DEEPSEEK_API_KEY="+r.APIKey)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Result{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	msgs := make(chan rpcMessage, 64)
	readErr := make(chan error, 1)
	go func() {
		reader := bufio.NewReaderSize(stdout, 1<<20)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				var msg rpcMessage
				if json.Unmarshal(line, &msg) == nil {
					msgs <- msg
				}
			}
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	nextID := 1
	var notifications []rpcMessage
	call := func(method string, params any) (json.RawMessage, error) {
		id := nextID
		nextID++
		rawID, _ := json.Marshal(id)
		body := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
		if params != nil {
			body["params"] = params
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(stdin, string(encoded)+"\n"); err != nil {
			return nil, err
		}
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case err := <-readErr:
				if err == io.EOF {
					return nil, fmt.Errorf("dsh exited: %s", stderr.String())
				}
				return nil, err
			case msg := <-msgs:
				if msg.Method != "" {
					notifications = append(notifications, msg)
					continue
				}
				if string(msg.ID) != string(rawID) {
					continue
				}
				if msg.Error != nil {
					return nil, fmt.Errorf("dsh %s: %s", method, msg.Error.Message)
				}
				return msg.Result, nil
			}
		}
	}

	maxTokens := r.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 32768
	}
	provider := r.Provider
	if provider == "" {
		provider = "deepseek-official"
	}
	model := r.Model
	if model == "" {
		model = "deepseek-flash"
	}
	if _, err := call("initialize", map[string]any{
		"cwd": workspace, "provider": provider, "model": model, "maxTokens": maxTokens,
	}); err != nil {
		return Result{}, err
	}
	if _, err := call("session/prompt", map[string]any{
		"sessionId":     job.ID,
		"contentBlocks": []map[string]any{{"type": "text", "text": job.Prompt}},
	}); err != nil {
		return Result{}, err
	}
	if _, err := call("session/wait", map[string]any{"sessionId": job.ID}); err != nil {
		return Result{}, err
	}
	_, _ = call("shutdown", nil)
	return Result{Output: assistantText(notifications)}, nil
}

func assistantText(notifications []rpcMessage) string {
	var texts []string
	for _, msg := range notifications {
		if msg.Method != "session.event" {
			continue
		}
		var params struct {
			Event struct {
				Type string `json:"type"`
				Data struct {
					Message struct {
						Content []struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"content"`
					} `json:"message"`
				} `json:"data"`
			} `json:"event"`
		}
		if json.Unmarshal(msg.Params, &params) != nil {
			continue
		}
		if params.Event.Type != "assistant/message" {
			continue
		}
		for _, block := range params.Event.Data.Message.Content {
			if block.Type == "text" && block.Text != "" {
				texts = append(texts, block.Text)
			}
		}
	}
	return strings.Join(texts, "\n")
}
