// internal/stagetest/harness_test.go
package stagetest_test

import (
	"context"
	"testing"

	"google.golang.org/adk/v2/agent/llmagent"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestRunAgentCapturesOutputKey(t *testing.T) {
	ctx := context.Background()
	// A trivial agent that echoes and saves to a state key.
	ag, err := llmagent.New(llmagent.Config{
		Name:            "echo",
		Model:           &stagetest.CannedLLM{Text: "hello-out"},
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Brief,
		Instruction:     "Echo back whatever is asked.",
	})
	if err != nil {
		t.Fatal(err)
	}

	state := stagetest.RunAgent(ctx, t, ag, "u1", "ping", nil)
	if got := stagetest.StateString(t, state, keys.Brief); got != "hello-out" {
		t.Errorf("brief = %q, want hello-out", got)
	}
}
