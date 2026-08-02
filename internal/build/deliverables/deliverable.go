// Package deliverables defines the swappable Deliverable contract every build
// vertical implements.
package deliverables

import (
	"context"
	"encoding/json"
)

// BuildInput is the data handed to a Deliverable's Build. Draft is the settled
// text from the eval loop; the JSON fields are the upstream stage outputs.
type BuildInput struct {
	Brief    json.RawMessage
	Plan     json.RawMessage
	Research json.RawMessage
	Brand    json.RawMessage
	Draft    string
}

// Artifact is a finished deliverable: a named blob of content.
type Artifact struct {
	Name    string
	Content string
}

// Deliverable is the contract every vertical satisfies so the build graph can
// route to any of them identically.
type Deliverable interface {
	Name() string
	Build(ctx context.Context, in BuildInput) (*Artifact, error)
}
