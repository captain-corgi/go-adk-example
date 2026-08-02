// internal/stages/research/research_test.go
package research_test

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/adk/v2/memory"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestResearchReadsBriefAndWritesOutput(t *testing.T) {
	b, err := brain.New(t.TempDir(), memory.InMemoryService())
	if err != nil {
		t.Fatal(err)
	}
	canned := `{"findings":["f1","f2"],"relevant_rules":[]}`
	ag, err := research.New(stages.Config{Model: &stagetest.CannedLLM{Text: canned}, Brand: ""}, b)
	if err != nil {
		t.Fatal(err)
	}
	seed := map[string]any{keys.Brief: `{"product":"Widget","goal":"launch","audience":"builders","tone":"bold","constraints":[]}`}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go", seed)

	out := stagetest.StateString(t, state, keys.Research)
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("research_output not JSON: %v\n%s", err, out)
	}
}
