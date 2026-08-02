// internal/stages/synthesis/synthesis.go
package synthesis

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
)

// New builds the synthesis stage: merges brief, angles, and research into a
// concrete campaign plan.
func New(cfg stages.Config) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:            "synthesis",
		Model:           cfg.Model,
		Description:     "Merges brief, ideation, and research into a campaign plan.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Plan,
		Instruction: cfg.Brand + `
You are the SYNTHESIS stage of a marketing engine.
Brief (JSON): {brief_output}
Angles (JSON): {ideation_output}
Research (JSON): {research_output}

Produce ONE concrete campaign plan as JSON with EXACTLY:
{"hook": string, "offer": string, "message_hierarchy": [string], "cta": string, "deliverable": "landing_page"}.
Return ONLY the JSON, no prose.`,
	})
}
