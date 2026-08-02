// internal/stages/stage.go
// Package stages holds shared types for pipeline stage constructors.
package stages

import "google.golang.org/adk/v2/model"

// Config is the common input to every stage constructor.
type Config struct {
	Model model.LLM
	Brand string // frozen gBrain context, prepended to the Instruction
}
