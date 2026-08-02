// internal/e2e/e2e_test.go
package e2e_test

import (
	"context"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/v2/artifact"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/build/graph"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stages/results"
	"github.com/captain-corgi/go-adk-example/internal/stages/signoff"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

// TestPipelineShipsLandingPage is the end-to-end smoke test: one raw idea in,
// one landing-page deliverable out, with fake models and AUTO_APPROVE (no real
// API key, no human in the loop). It proves the 7-stage sequentialagent chains
// its state keys end to end and ships the landing_page.md artifact.
func TestPipelineShipsLandingPage(t *testing.T) {
	ctx := context.Background()
	const app = "engine_e2e"

	// Fakes: the four JSON LLM stages (brief/ideation/research/synthesis)
	// return valid JSON; the drafter returns a valid landing page; the checker
	// never calls exit_loop (max_iter=2 is fine — finalize ships the draft).
	jsonLLM := &stagetest.CannedLLM{Text: `{"ok":true}`}
	drafterLLM := &stagetest.CannedLLM{Text: "# Hero\nOffer: free\nCTA: Sign up"}
	checkerLLM := &stagetest.CannedLLM{Text: `{"critique":"x","fixes":[]}`}
	buildModels := routing.NewModels(checkerLLM, drafterLLM) // strong=checker, cheap=drafter

	mem := memory.InMemoryService()
	art := artifact.InMemoryService()
	b, err := brain.New(t.TempDir(), mem)
	if err != nil {
		t.Fatal(err)
	}

	briefAg, err := brief.New(stages.Config{Model: jsonLLM})
	if err != nil {
		t.Fatal(err)
	}
	ideationAg, err := ideation.New(stages.Config{Model: jsonLLM})
	if err != nil {
		t.Fatal(err)
	}
	researchAg, err := research.New(stages.Config{Model: jsonLLM}, b)
	if err != nil {
		t.Fatal(err)
	}
	synthesisAg, err := synthesis.New(stages.Config{Model: jsonLLM})
	if err != nil {
		t.Fatal(err)
	}
	signoffAg, err := signoff.New(true) // AUTO_APPROVE — skip the HITL pause
	if err != nil {
		t.Fatal(err)
	}
	buildAg, err := graph.New(buildModels, 2)
	if err != nil {
		t.Fatal(err)
	}
	resultsAg, err := results.New(mem)
	if err != nil {
		t.Fatal(err)
	}

	// The four JSON stages use jsonLLM directly; build uses its own
	// drafter/checker models via graph.New. All seven share one session.
	pipeline, err := sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name: "marketing_engine",
			SubAgents: []agent.Agent{
				briefAg, ideationAg, researchAg, synthesisAg, signoffAg, buildAg, resultsAg,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ss := session.InMemoryService()
	created, err := ss.Create(ctx, &session.CreateRequest{AppName: app, UserID: "u1"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess := created.Session

	r, err := runner.New(runner.Config{
		AppName:         app,
		Agent:           pipeline,
		SessionService:  ss,
		ArtifactService: art,
		MemoryService:   mem,
	})
	if err != nil {
		t.Fatal(err)
	}

	state := map[string]any{}
	for ev, err := range r.Run(ctx, "u1", sess.ID(),
		genai.NewContentFromText("A no-code analytics tool for indie hackers", genai.RoleUser),
		agent.RunConfig{}) {
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		for k, v := range ev.Actions.StateDelta {
			state[k] = v
		}
	}

	if _, ok := state[keys.Build]; !ok {
		t.Fatalf("build_output not produced; state=%v", state)
	}
	if _, ok := state[keys.Results]; !ok {
		t.Errorf("results_output not produced (pipeline did not complete); state=%v", state)
	}

	// The landing-page deliverable must be persisted as a retrievable artifact.
	loaded, err := art.Load(ctx, &artifact.LoadRequest{
		AppName:   app,
		UserID:    "u1",
		SessionID: sess.ID(),
		FileName:  "landing_page.md",
	})
	if err != nil {
		t.Fatalf("landing_page.md artifact not saved: %v", err)
	}
	if loaded.Part == nil || loaded.Part.Text == "" {
		t.Error("saved landing_page.md artifact has no text content")
	}
}
