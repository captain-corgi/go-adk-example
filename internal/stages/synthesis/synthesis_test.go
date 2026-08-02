// internal/stages/synthesis/synthesis_test.go
package synthesis_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestSynthesisMergesInputsIntoPlan(t *testing.T) {
	canned := `{"hook":"h","offer":"o","message_hierarchy":["a"],"cta":"Sign up","deliverable":"landing_page"}`
	ag, err := synthesis.New(stages.Config{Model: &stagetest.CannedLLM{Text: canned}, Brand: ""})
	if err != nil {
		t.Fatal(err)
	}
	seed := map[string]any{
		keys.Brief:    `{"product":"Widget"}`,
		keys.Ideation: `{"angles":[]}`,
		keys.Research: `{"findings":[]}`,
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go", seed)

	out := stagetest.StateString(t, state, keys.Plan)
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("plan_output not JSON: %v\n%s", err, out)
	}
}
