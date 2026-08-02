// internal/build/graph/graph.go
package graph

import (
	"encoding/json"
	"fmt"
	"log"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"

	"github.com/captain-corgi/go-adk-example/internal/build/deliverables"
	"github.com/captain-corgi/go-adk-example/internal/build/deliverables/landingpage"
	"github.com/captain-corgi/go-adk-example/internal/build/loop"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
)

// New builds the "build" stage: a workflow graph that runs the landing-page
// eval loop, then finalizes (validate + save artifact + publish build_output).
// The fan-out/fan-in structure is in place even at N=1 so Phase 1 only adds edges.
func New(models routing.Models, maxIter int) (agent.Agent, error) {
	lp := landingpage.New()
	loopAgent, err := loop.New(lp, models, maxIter)
	if err != nil {
		return nil, fmt.Errorf("build loop: %w", err)
	}

	loopNode, err := workflow.NewAgentNode(loopAgent, workflow.NodeConfig{})
	if err != nil {
		return nil, fmt.Errorf("loop node: %w", err)
	}
	joinNode := workflow.NewJoinNode("build_join")
	finalizeNode := workflow.NewFunctionNode[any, *session.Event]("finalize", finalize(lp), workflow.NodeConfig{})

	eb := workflow.NewEdgeBuilder()
	eb.AddFanOut(workflow.Start, loopNode)
	eb.AddFanIn(joinNode, loopNode)
	eb.Add(joinNode, finalizeNode)

	return workflowagent.New(workflowagent.Config{
		Name:        "build",
		Description: "Runs the deliverable eval loop and finalizes artifacts.",
		Edges:       eb.Build(),
		SubAgents:   []agent.Agent{loopAgent},
	})
}

// finalize is the function-node body: read the settled draft + plan + verdict
// from state, validate via the deliverable, save the artifact, and return a
// *session.Event carrying build_output in StateDelta.
//
// Why it returns *session.Event (verified against adk v2.1.0): a FunctionNode
// does NOT propagate ctx.State().Set into its emitted event — Run builds a fresh
// event with only Output set and discards ctx.actions.StateDelta. But Run
// special-cases a *session.Event return (function_node.go:337-343) and yields it
// directly, StateDelta intact, so build_output reaches the runner/harness.
func finalize(lp *landingpage.LandingPage) func(ctx agent.Context, _ any) (*session.Event, error) {
	return func(ctx agent.Context, _ any) (*session.Event, error) {
		draftStr := stateString(ctx, keys.Draft)
		planStr := stateString(ctx, keys.Plan)

		quality := "max_iter_reached"
		if stateString(ctx, keys.Verdict) == "pass" {
			quality = "approved"
		}

		art, err := lp.Build(ctx, deliverables.BuildInput{
			Draft: draftStr,
			Plan:  json.RawMessage(planStr),
		})
		if err != nil {
			// Non-converged drafts may fail validation; still ship what we have.
			log.Printf("build: deliverable validation failed (shipping best draft): %v", err)
			art = &deliverables.Artifact{Name: "landing_page.md", Content: draftStr}
		}
		if _, err := ctx.Artifacts().Save(ctx, art.Name, genai.NewPartFromText(art.Content)); err != nil {
			return nil, fmt.Errorf("save artifact: %w", err)
		}

		out := map[string]any{"landing_page": art.Name, "quality": quality}
		payload, _ := json.Marshal(out)
		ev := session.NewEvent(ctx, ctx.InvocationID())
		ev.Actions = session.EventActions{StateDelta: map[string]any{
			keys.Build:    string(payload),
			keys.Verdict:  "", // reset for the next run's eval loop
			keys.Critique: "", // reset: a stale critique would steer the next run's first draft
			keys.Draft:    "", // reset: never ship a previous run's draft if the next drafter is empty
		}}
		return ev, nil
	}
}

func stateString(ctx agent.Context, key string) string {
	v, _ := ctx.State().Get(key)
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
