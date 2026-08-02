// internal/stages/brief/brief_test.go
package brief_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestBriefWritesStructuredJSON(t *testing.T) {
	canned := `{"product":"Widget","goal":"launch","audience":"builders","tone":"bold","constraints":[]}`
	ag, err := brief.New(stages.Config{
		Model: &stagetest.CannedLLM{Text: canned},
		Brand: "BRAND=acme\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1",
		"A no-code analytics tool for indie hackers", nil)

	out := stagetest.StateString(t, state, keys.Brief)
	var b map[string]any
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatalf("brief_output not valid JSON: %v\nraw=%s", err, out)
	}
	if b["product"] != "Widget" {
		t.Errorf("product = %v, want Widget", b["product"])
	}
}
