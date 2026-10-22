//go:build integration

package dshgw

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSDKRunnerSmoke(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}
	binary := os.Getenv("GP_DSH_BINARY")
	if binary == "" {
		binary = "dsh.cmd"
	}
	runner := SDKRunner{
		Binary: binary, Profile: "sdk", Provider: "deepseek-official", Model: "deepseek-flash",
		APIKey: os.Getenv("DEEPSEEK_API_KEY"), MaxTokens: 2048,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	result, err := runner.Run(ctx, Job{
		ID:     "gp-smoke-1",
		Prompt: `Reply with exactly this JSON array and nothing else: [{"code":"smoke","title":"smoke"}]. Do not call tools.`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Output, "smoke") {
		t.Fatalf("unexpected output: %s", result.Output)
	}
}
