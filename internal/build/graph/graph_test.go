// internal/build/graph/graph_test.go
package graph_test

import (
	"context"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/artifact"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"

	"github.com/captain-corgi/go-adk-example/internal/build/graph"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestBuildGraphProducesBuildOutput(t *testing.T) {
	// Drafter returns a valid landing page; checker never exits (max_iter).
	drafter := &stagetest.CannedLLM{Text: "# Hero\nOffer: free\nCTA: Sign up"}
	checker := &stagetest.CannedLLM{Text: `{"critique":"x","fixes":[]}`}
	m := routing.NewModels(checker, drafter) // strong=checker, cheap=drafter

	ag, err := graph.New(m, 2)
	if err != nil {
		t.Fatal(err)
	}

	const app = "engine_test"
	ctx := context.Background()
	ss := session.InMemoryService()
	as := artifact.InMemoryService()

	created, err := ss.Create(ctx, &session.CreateRequest{AppName: app, UserID: "u1"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess := created.Session

	// Seed the plan the drafter reads via templating. AppendEvent applies the
	// StateDelta to the stored session, so the runner's Get() sees it.
	sev := session.NewEvent(ctx, "seed")
	sev.Author = "user"
	sev.Actions = session.EventActions{StateDelta: map[string]any{keys.Plan: `{"deliverable":"landing_page"}`}}
	if err := ss.AppendEvent(ctx, sess, sev); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r, err := runner.New(runner.Config{AppName: app, Agent: ag, SessionService: ss, ArtifactService: as})
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]any{}
	for ev, err := range r.Run(ctx, "u1", sess.ID(), genai.NewContentFromText("build", genai.RoleUser), agent.RunConfig{}) {
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		for k, v := range ev.Actions.StateDelta {
			got[k] = v
		}
	}

	if _, ok := got[keys.Build]; !ok {
		t.Fatalf("build_output not written; state=%v", got)
	}

	// The deliverable must also be persisted as a retrievable artifact.
	loaded, err := as.Load(ctx, &artifact.LoadRequest{AppName: app, UserID: "u1", SessionID: sess.ID(), FileName: "landing_page.md"})
	if err != nil {
		t.Fatalf("artifact not saved: %v", err)
	}
	if loaded.Part == nil || loaded.Part.Text == "" {
		t.Error("saved artifact has no text content")
	}
}
