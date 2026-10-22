package dshgw

import (
	"encoding/json"
	"testing"
)

func TestAssistantText(t *testing.T) {
	params, _ := json.Marshal(map[string]any{
		"sessionId": "s1",
		"event": map[string]any{
			"type": "assistant/message",
			"data": map[string]any{
				"message": map[string]any{
					"content": []map[string]any{{"type": "text", "text": "hello"}, {"type": "text", "text": "world"}},
				},
			},
		},
	})
	got := assistantText([]rpcMessage{{Method: "session.event", Params: params}})
	if got != "hello\nworld" {
		t.Fatalf("got %q", got)
	}
}
