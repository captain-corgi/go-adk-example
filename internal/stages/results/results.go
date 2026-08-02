// internal/stages/results/results.go
package results

import (
	"encoding/json"
	"log"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/captain-corgi/go-adk-example/internal/keys"
)

// New builds the results stage: ingests the session into gBrain memory (the
// feedback-loop seam; enriched in Phase 3) and publishes results_output.
func New(mem memory.Service) (agent.Agent, error) {
	node := workflow.NewFunctionNode[any, *session.Event]("record_results",
		func(ctx agent.Context, _ any) (*session.Event, error) {
			if mem != nil {
				if err := mem.AddSessionToMemory(ctx, ctx.Session()); err != nil {
					// Non-fatal: the feedback seam is best-effort in Phase 0.
					log.Printf("results: AddSessionToMemory failed: %v", err)
				}
			}
			// Publish results_output by RETURNING a *session.Event: the plain
			// FunctionNode yields it verbatim (function_node.go:337-343), so its
			// StateDelta reaches the runner/harness. (ctx.State().Set would be
			// unobservable from a function node.) The published status reflects
			// the build quality: "degraded" when the build did not reach
			// "approved", else "complete"; the build metadata is carried along.
			out := map[string]any{}
			status := "complete"
			if raw, _ := ctx.State().Get(keys.Build); raw != nil {
				if s, ok := raw.(string); ok && s != "" {
					var build map[string]any
					if err := json.Unmarshal([]byte(s), &build); err == nil {
						out["build"] = build
						if q, _ := build["quality"].(string); q != "" && q != "approved" {
							status = "degraded"
						}
					}
				}
			}
			out["status"] = status
			payload, _ := json.Marshal(out)
			ev := session.NewEvent(ctx, ctx.InvocationID())
			ev.Actions = session.EventActions{StateDelta: map[string]any{keys.Results: string(payload)}}
			return ev, nil
		}, workflow.NodeConfig{})

	eb := workflow.NewEdgeBuilder()
	eb.Add(workflow.Start, node)
	return workflowagent.New(workflowagent.Config{
		Name:        "results",
		Description: "Persists results and writes the gBrain memory feedback seam.",
		Edges:       eb.Build(),
	})
}
