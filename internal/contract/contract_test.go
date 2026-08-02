// internal/contract/contract_test.go
package contract_test

import (
	"context"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/memory"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

// TestEachStagePublishesItsKey is the swappability contract: each llmagent
// stage, fed its input key, writes its output key. Guards that no stage
// silently changed its key name. (The workflow stages signoff/build/results are
// unit-tested in their own packages.)
func TestEachStagePublishesItsKey(t *testing.T) {
	ctx := context.Background()
	m := routing.NewModels(&stagetest.CannedLLM{Text: `{"ok":true}`}, &stagetest.CannedLLM{Text: `{"ok":true}`})
	b, err := brain.New(t.TempDir(), memory.InMemoryService())
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		build   func() (agent.Agent, error)
		seed    map[string]any
		wantKey string
	}{
		{"brief", func() (agent.Agent, error) { return brief.New(stages.Config{Model: m.Cheap()}) }, nil, keys.Brief},
		{"ideation", func() (agent.Agent, error) { return ideation.New(stages.Config{Model: m.Cheap()}) }, map[string]any{keys.Brief: `{}`}, keys.Ideation},
		{"research", func() (agent.Agent, error) { return research.New(stages.Config{Model: m.Cheap()}, b) }, map[string]any{keys.Brief: `{}`}, keys.Research},
		{"synthesis", func() (agent.Agent, error) { return synthesis.New(stages.Config{Model: m.Strong()}) }, map[string]any{keys.Brief: `{}`, keys.Ideation: `{}`, keys.Research: `{}`}, keys.Plan},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ag, err := c.build()
			if err != nil {
				t.Fatal(err)
			}
			state := stagetest.RunAgent(ctx, t, ag, "u1", "x", c.seed)
			if _, ok := state[c.wantKey]; !ok {
				t.Errorf("%s did not publish %s; state=%v", c.name, c.wantKey, state)
			}
		})
	}
}
