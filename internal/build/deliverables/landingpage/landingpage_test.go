// internal/build/deliverables/landingpage/landingpage_test.go
package landingpage_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/build/deliverables"
	"github.com/captain-corgi/go-adk-example/internal/build/deliverables/landingpage"
)

func TestBuildAcceptsValidDraft(t *testing.T) {
	lp := landingpage.New()
	art, err := lp.Build(context.Background(), deliverables.BuildInput{
		Draft: "# Hero headline\n\nOffer: 50% off the first month.\n\nCTA: Start free →",
	})
	if err != nil {
		t.Fatalf("valid draft rejected: %v", err)
	}
	if art.Name != "landing_page.md" {
		t.Errorf("Name = %q, want landing_page.md", art.Name)
	}
	if art.Content == "" {
		t.Error("Content must not be empty")
	}
}

func TestBuildRejectsDraftMissingCTA(t *testing.T) {
	lp := landingpage.New()
	_, err := lp.Build(context.Background(), deliverables.BuildInput{
		Draft: "# Hero headline\n\nOffer: 50% off.",
	})
	if err == nil {
		t.Fatal("draft without CTA must be rejected")
	}
}

func TestLoopConfigAccessors(t *testing.T) {
	lp := landingpage.New()
	if lp.Name() != "landing_page" {
		t.Errorf("Name = %q", lp.Name())
	}
	if lp.DrafterInstruction() == "" || lp.CheckerInstruction() == "" {
		t.Error("loop instructions must be non-empty")
	}
}
