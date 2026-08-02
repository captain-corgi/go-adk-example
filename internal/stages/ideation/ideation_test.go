// internal/stages/ideation/ideation_test.go
package ideation_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestIdeationReadsBriefWritesAngles(t *testing.T) {
	canned := `{"angles":[{"name":"a","score":9,"rationale":"x"}]}`
	ag, err := ideation.New(stages.Config{Model: &stagetest.CannedLLM{Text: canned}, Brand: "BRAND=acme\n"})
	if err != nil {
		t.Fatal(err)
	}
	seed := map[string]any{keys.Brief: `{"product":"Widget","goal":"launch","audience":"builders","tone":"bold","constraints":[]}`}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go", seed)

	out := stagetest.StateString(t, state, keys.Ideation)
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("ideation_output not JSON: %v\n%s", err, out)
	}
	if _, ok := v["angles"]; !ok {
		t.Errorf("ideation_output missing 'angles': %s", out)
	}
}
