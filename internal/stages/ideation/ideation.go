// internal/stages/ideation/ideation.go
package ideation

import (
	"fmt"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
)

// New builds the ideation stage: generates angles from the brief, scored
// against brand voice/ICP, keeping the top few.
func New(cfg stages.Config) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:            "ideation",
		Model:           cfg.Model,
		Description:     "Generates and scores campaign angles against the brief and brand.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Ideation,
		Instruction: cfg.Brand + fmt.Sprintf(`
You are the IDEATION stage of a marketing engine.
The structured brief (JSON):
{%s}

Generate roughly six campaign angles, score each (1-10) against the brand voice and the
brief's audience, and keep the top 2-3. Return JSON with EXACTLY:
{"angles": [{"name": string, "score": number, "rationale": string}]}.
Return ONLY the JSON, no prose.`, keys.Brief),
	})
}
