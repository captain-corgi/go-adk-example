// internal/stages/brief/brief_test.go
package brief_test

import (
	"context"
	"encoding/json"
	"strings"
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

// TestBriefReceivesRawIdea pins the load-bearing adk behavior the brief stage
// relies on: IncludeContentsNone means "current turn only", NOT "send nothing".
// The runner appends the user's message as a user-authored session event before
// the agent runs, so the brief model still receives the raw idea as
// req.Contents[0]. A CannedLLM hides this (it ignores the request), so a
// RecordingLLM is used to capture the request and assert the idea text is in it.
func TestBriefReceivesRawIdea(t *testing.T) {
	const idea = "A no-code analytics tool for indie hackers"
	m := &stagetest.RecordingLLM{Text: `{"product":"Widget","goal":"launch","audience":"builders","tone":"bold","constraints":[]}`}
	ag, err := brief.New(stages.Config{Model: m, Brand: "BRAND=acme\n"})
	if err != nil {
		t.Fatal(err)
	}
	stagetest.RunAgent(context.Background(), t, ag, "u1", idea, nil)

	if len(m.Requests) == 0 {
		t.Fatal("brief model was never called")
	}
	if got := stagetest.RequestText(m.Requests[0]); !strings.Contains(got, "no-code analytics tool") {
		t.Errorf("raw idea not delivered to brief model under IncludeContentsNone;\n request contents text = %q", got)
	}
}
