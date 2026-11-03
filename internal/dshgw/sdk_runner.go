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
	session, err := r.startSession()
	if err != nil {
		return Result{}, err
	}
	defer session.Close()

	provider, model, maxTokens := r.defaults()
	if _, err := session.call(ctx, "initialize", map[string]any{
		"cwd": session.workspace, "provider": provider, "model": model, "maxTokens": maxTokens,
	}); err != nil {
		return Result{}, err
	}
	if _, err := session.call(ctx, "session/prompt", map[string]any{
		"sessionId":     job.ID,
		"contentBlocks": []map[string]any{{"type": "text", "text": job.Prompt}},
	}); err != nil {
		return Result{}, err
	}
	if _, err := session.call(ctx, "session/wait", map[string]any{"sessionId": job.ID}); err != nil {
		return Result{}, err
	}
	_, _ = session.call(ctx, "shutdown", nil)
	return session.result(model), nil
}

// defaults applies the runner's documented model/provider fallbacks.
func (r SDKRunner) defaults() (provider, model string, maxTokens int) {
	maxTokens = r.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 32768
	}
	provider = r.Provider
	if provider == "" {
		provider = "deepseek-official"
	}
	model = r.Model
	if model == "" {
		model = "deepseek-flash"
	}
	return provider, model, maxTokens
}

// rpcSession owns one DSH SDK process together with its request state.
type rpcSession struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stderr        *strings.Builder
	workspace     string
	home          string
	msgs          chan rpcMessage
	readErr       chan error
	nextID        int
	notifications []rpcMessage
}

func (r SDKRunner) startSession() (*rpcSession, error) {
	workspace, err := os.MkdirTemp("", "gp-dsh-job-*")
	if err != nil {
		return nil, err
	}
	home, err := os.MkdirTemp("", "gp-dsh-home-*")
	if err != nil {
		_ = os.RemoveAll(workspace)
		return nil, err
	}
	session, err := r.startProcess(workspace, home)
	if err != nil {
		_ = os.RemoveAll(home)
		_ = os.RemoveAll(workspace)
		return nil, err
	}
	return session, nil
}

func (r SDKRunner) startProcess(workspace, home string) (*rpcSession, error) {
	binary := r.Binary
	if binary == "" {
		binary = "dsh"
	}
	profile := r.Profile
	if profile == "" {
		profile = "sdk"
	}
	cmd := exec.Command(binary, "--profile", profile)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), "DSH_HOME="+home)
	if r.APIKey != "" {
		cmd.Env = append(cmd.Env, "DEEPSEEK_API_KEY="+r.APIKey)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	session := &rpcSession{
		cmd:       cmd,
		stdin:     stdin,
		stderr:    stderr,
		workspace: workspace,
		home:      home,
		msgs:      make(chan rpcMessage, 64),
		readErr:   make(chan error, 1),
		nextID:    1,
	}
	go session.readLoop(stdout)
	return session, nil
}

// Close kills the process (if still alive), reaps it, and removes both
// disposable directories. The order matches the previous deferred cleanup.
func (s *rpcSession) Close() {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	}
	_ = os.RemoveAll(s.home)
	_ = os.RemoveAll(s.workspace)
}

func (s *rpcSession) readLoop(stdout io.Reader) {
	reader := bufio.NewReaderSize(stdout, 1<<20)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var msg rpcMessage
			if json.Unmarshal(line, &msg) == nil {
				s.msgs <- msg
			}
		}
		if err != nil {
			s.readErr <- err
			return
		}
	}
}

// call issues one JSON-RPC request and returns the response matching its ID,
// buffering notifications for the final assistant output.
func (s *rpcSession) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := s.nextID
	s.nextID++
	rawID, _ := json.Marshal(id)
	body := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(s.stdin, string(encoded)+"\n"); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-s.readErr:
			if err == io.EOF {
				return nil, fmt.Errorf("dsh exited: %s", s.stderr.String())
			}
			return nil, err
		case msg := <-s.msgs:
			if msg.Method != "" {
				s.notifications = append(s.notifications, msg)
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

func (s *rpcSession) result(model string) Result {
	text := assistantText(s.notifications)
	if text == "" {
		text = rawNotifications(s.notifications)
	}
	tokensIn, tokensOut, cacheRead := assistantUsage(s.notifications)
	return Result{Output: text, TokensIn: tokensIn, TokensOut: tokensOut, CacheReadTokens: cacheRead, Model: model}
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

func rawNotifications(notifications []rpcMessage) string {
	buf := make([]byte, 0, 4096)
	for _, msg := range notifications {
		encoded, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		buf = append(buf, encoded...)
		buf = append(buf, '\n')
	}
	return string(buf)
}

// assistantUsage sums provider-reported usage across all model attempts in
// one DSH session. Token counts are authoritative only when the provider
// emitted them; absent usage stays zero.
func assistantUsage(notifications []rpcMessage) (int64, int64, int64) {
	var in, out, cache int64
	for _, msg := range notifications {
		if msg.Method != "session.event" {
			continue
		}
		var params struct {
			Event struct {
				Type string `json:"type"`
				Data struct {
					Usage struct {
						InputTokens     int64 `json:"inputTokens"`
						OutputTokens    int64 `json:"outputTokens"`
						CacheReadTokens int64 `json:"cacheReadTokens"`
					} `json:"usage"`
				} `json:"data"`
			} `json:"event"`
		}
		if json.Unmarshal(msg.Params, &params) != nil {
			continue
		}
		if params.Event.Type != "assistant/attempt" && params.Event.Type != "assistant/message" {
			continue
		}
		in += params.Event.Data.Usage.InputTokens
		out += params.Event.Data.Usage.OutputTokens
		cache += params.Event.Data.Usage.CacheReadTokens
	}
	return in, out, cache
}
