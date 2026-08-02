// internal/stages/results/results_test.go
package results_test

import (
	"context"
	"testing"

	"google.golang.org/adk/v2/memory"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages/results"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestResultsWritesOutputAndIngestsMemory(t *testing.T) {
	mem := memory.InMemoryService()
	ag, err := results.New(mem)
	if err != nil {
		t.Fatal(err)
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "done",
		map[string]any{keys.Build: `{"landing_page":"landing_page.md","quality":"approved"}`})

	if _, ok := state[keys.Results]; !ok {
		t.Fatalf("results_output not written; state=%v", state)
	}
}
