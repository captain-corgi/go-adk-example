// internal/routing/routing_test.go
package routing_test

import (
	"context"
	"iter"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/routing"
	"google.golang.org/adk/v2/model"
)

// stubLLM is a zero-config model.LLM used only to test identity routing.
type stubLLM struct{ name string }

func (s *stubLLM) Name() string { return s.name }
func (s *stubLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(func(*model.LLMResponse, error) bool) {}
}

var _ model.LLM = (*stubLLM)(nil)

func TestModelsResolvesRoles(t *testing.T) {
	strong := &stubLLM{name: "strong"}
	cheap := &stubLLM{name: "cheap"}
	m := routing.NewModels(strong, cheap)

	if m.Strong() != strong {
		t.Fatal("Strong() must return the strong model instance")
	}
	if m.Cheap() != cheap {
		t.Fatal("Cheap() must return the cheap model instance")
	}
	if m.For(routing.StrongRole) != strong || m.For(routing.CheapRole) != cheap {
		t.Fatal("For(role) must resolve each role to its model")
	}
}
