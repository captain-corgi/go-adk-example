// internal/build/loop/loop.go
package loop

import (
	"fmt"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/agent/workflowagents/loopagent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/exitlooptool"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
)

// LoopSpec is the per-vertical config the loop needs to build its agents.
// *landingpage.LandingPage satisfies it; Phase 1 formalizes this as Loopable.
type LoopSpec interface {
	DrafterInstruction() string
	CheckerInstruction() string
	DraftKey() string
	CritiqueKey() string
}

// New builds a loopagent over [drafter, checker]. The drafter produces the
// deliverable; the checker calls exitlooptool on pass. Both are history-less,
// state-passing agents (IncludeContentsNone) to avoid the openaimodel
// multi-turn bug.
func New(spec LoopSpec, models routing.Models, maxIter int) (agent.Agent, error) {
	if maxIter < 1 {
		maxIter = 1
	}
	drafter, err := llmagent.New(llmagent.Config{
		Name:            "drafter",
		Model:           models.Cheap(),
		Description:     "Produces a deliverable draft, improved from prior critique.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       spec.DraftKey(),
		Instruction: fmt.Sprintf(`%s

Campaign plan (JSON):
{%s}

Prior critique, if any:
{%s?}

Return ONLY the deliverable Markdown.`, spec.DrafterInstruction(), keys.Plan, spec.CritiqueKey()),
	})
	if err != nil {
		return nil, fmt.Errorf("drafter: %w", err)
	}

	exitTool, err := exitlooptool.New()
	if err != nil {
		return nil, fmt.Errorf("exitlooptool: %w", err)
	}

	checker, err := llmagent.New(llmagent.Config{
		Name:               "checker",
		Model:              models.Strong(),
		Description:        "Reviews the draft; calls exit_loop when it passes.",
		IncludeContents:    llmagent.IncludeContentsNone,
		OutputKey:          spec.CritiqueKey(),
		Tools:              []tool.Tool{exitTool},
		AfterToolCallbacks: []llmagent.AfterToolCallback{verdictOnExit},
		Instruction: fmt.Sprintf(`%s

Draft to review:
{%s}`, spec.CheckerInstruction(), spec.DraftKey()),
	})
	if err != nil {
		return nil, fmt.Errorf("checker: %w", err)
	}

	return loopagent.New(loopagent.Config{
		MaxIterations: uint(maxIter),
		AgentConfig: agent.Config{
			Name:      "eval_loop",
			SubAgents: []agent.Agent{drafter, checker},
		},
	})
}

// verdictOnExit stamps landing_verdict=pass when the exit_loop tool runs.
func verdictOnExit(ctx agent.Context, tl tool.Tool, _, result map[string]any, err error) (map[string]any, error) {
	if err == nil && tl != nil && tl.Name() == "exit_loop" {
		_ = ctx.State().Set(keys.Verdict, "pass")
	}
	return result, err
}
