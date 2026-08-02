// internal/build/loop/loop_test.go
package loop_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/build/deliverables/landingpage"
	"github.com/captain-corgi/go-adk-example/internal/build/loop"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

// specStub adapts *landingpage.LandingPage to loop.LoopSpec for the test.
type specStub struct{ lp *landingpage.LandingPage }

func (s specStub) DrafterInstruction() string { return s.lp.DrafterInstruction() }
func (s specStub) CheckerInstruction() string { return s.lp.CheckerInstruction() }
func (s specStub) DraftKey() string           { return s.lp.DraftKey() }
func (s specStub) CritiqueKey() string        { return s.lp.CritiqueKey() }

func TestLoopExitsWhenCheckerPasses(t *testing.T) {
	// Drafter always returns a valid draft; checker critiques once, then calls exit_loop.
	drafterModel := &stagetest.CannedLLM{Text: "# Hero\nOffer: free\nCTA: Sign up"}
	checkerModel := &stagetest.ScriptedLLM{Responses: []string{
		`{"critique":"add cta","fixes":["cta"]}`, // iter 1: fail
		"FC:exit_loop",                           // iter 2: pass
	}}
	m := routing.NewModels(checkerModel, drafterModel) // strong=checker, cheap=drafter

	ag, err := loop.New(specStub{lp: landingpage.New()}, m, 5)
	if err != nil {
		t.Fatal(err)
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go",
		map[string]any{keys.Plan: `{"deliverable":"landing_page"}`})

	if stagetest.StateString(t, state, keys.Verdict) != "pass" {
		t.Error("checker pass must set landing_verdict=pass")
	}
	if stagetest.StateString(t, state, keys.Draft) == "" {
		t.Error("drafter must write landing_draft")
	}
}

func TestLoopMaxIterShipsBestDraft(t *testing.T) {
	// Checker never calls exit_loop -> MaxIterations -> verdict unset.
	drafterModel := &stagetest.CannedLLM{Text: "# Hero\nOffer: free\nCTA: Sign up"}
	checkerModel := &stagetest.CannedLLM{Text: `{"critique":"nope","fixes":[]}`}
	m := routing.NewModels(checkerModel, drafterModel)

	ag, err := loop.New(specStub{lp: landingpage.New()}, m, 2)
	if err != nil {
		t.Fatal(err)
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go",
		map[string]any{keys.Plan: `{}`})

	if stagetest.StateString(t, state, keys.Draft) == "" {
		t.Error("best draft must still be written at max_iter")
	}
	if _, ok := state[keys.Verdict]; ok {
		t.Error("verdict must stay unset when the loop never converges")
	}
}
