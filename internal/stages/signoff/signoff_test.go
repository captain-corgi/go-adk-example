// internal/stages/signoff/signoff_test.go
package signoff_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages/signoff"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestAutoApproveCopiesPlanToSignoff(t *testing.T) {
	ag, err := signoff.New(true)
	if err != nil {
		t.Fatal(err)
	}
	plan := `{"hook":"h","offer":"o","cta":"Sign up","deliverable":"landing_page"}`
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "approve",
		map[string]any{keys.Plan: plan})

	if got := stagetest.StateString(t, state, keys.Signoff); got != plan {
		t.Errorf("signoff_output = %q, want the plan unchanged under AUTO_APPROVE", got)
	}
}
