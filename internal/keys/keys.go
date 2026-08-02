// Package keys holds the session state-key strings that couple pipeline stages.
// They are underscored identifiers because llmagent Instruction templating
// ({key}) only substitutes valid identifiers ^[a-zA-Z_][a-zA-Z0-9_]*$.
package keys

const (
	Brief    = "brief_output"
	Ideation = "ideation_output"
	Research = "research_output"
	Plan     = "plan_output"
	Signoff  = "signoff_output"
	Build    = "build_output"
	Results  = "results_output"

	Draft    = "landing_draft"
	Critique = "landing_critique"
	Verdict  = "landing_verdict"
	MemCtx   = "memory_context"
)
