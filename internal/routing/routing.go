// Package routing resolves a logical role to a backing model.LLM.
// Phase 0 routes both roles to the same model; the seam exists so Phase 5
// only changes the mapping.
package routing

import "google.golang.org/adk/v2/model"

type Role string

const (
	StrongRole Role = "strong" // judgment: synthesis, checker
	CheapRole  Role = "cheap"  // grunt: brief, ideation, research, drafter
)

// Models is the role→model registry. Fields are unexported; use the accessors.
type Models struct {
	strong model.LLM
	cheap  model.LLM
}

func NewModels(strong, cheap model.LLM) Models { return Models{strong: strong, cheap: cheap} }

func (m Models) Strong() model.LLM { return m.strong }
func (m Models) Cheap() model.LLM  { return m.cheap }

// For resolves a role; unknown roles fall back to Cheap.
func (m Models) For(role Role) model.LLM {
	if role == StrongRole {
		return m.strong
	}
	return m.cheap
}
