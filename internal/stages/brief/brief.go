// internal/stages/brief/brief.go
package brief

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
)

// New builds the brief stage: turns the user's raw idea into a structured brief.
func New(cfg stages.Config) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:            "brief",
		Model:           cfg.Model,
		Description:     "Converts a raw marketing idea into a structured campaign brief.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Brief,
		Instruction: cfg.Brand + `
You are the BRIEF stage of a marketing engine. The user's message is a raw marketing idea.
Convert it into a concise structured brief as JSON with EXACTLY these fields:
{"product": string, "goal": string, "audience": string, "tone": string, "constraints": [string]}.
Return ONLY the JSON object, no prose, no code fences.`,
	})
}
