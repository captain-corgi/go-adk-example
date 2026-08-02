// internal/build/deliverables/landingpage/landingpage.go
package landingpage

import (
	"context"
	"errors"
	"strings"

	"github.com/captain-corgi/go-adk-example/internal/build/deliverables"
	"github.com/captain-corgi/go-adk-example/internal/keys"
)

const artifactName = "landing_page.md"

// LandingPage is the Phase 0 deliverable.
type LandingPage struct{}

func New() *LandingPage { return &LandingPage{} }

func (l *LandingPage) Name() string { return "landing_page" }

// Build validates the settled draft has a hero, an offer, and a CTA, then
// returns it as the artifact. It does no model calls (the draft came from the
// eval loop); it is the deterministic finalize step.
func (l *LandingPage) Build(_ context.Context, in deliverables.BuildInput) (*deliverables.Artifact, error) {
	d := strings.ToLower(in.Draft)
	if !strings.Contains(d, "hero") && !startsWithHeading(in.Draft) {
		return nil, errors.New("landing page missing a hero section")
	}
	if !strings.Contains(d, "offer") && !strings.Contains(d, "%") && !strings.Contains(d, "free") {
		return nil, errors.New("landing page missing an offer")
	}
	if !strings.Contains(d, "cta") && !strings.Contains(d, "sign up") && !strings.Contains(d, "start") {
		return nil, errors.New("landing page missing a call to action")
	}
	return &deliverables.Artifact{Name: artifactName, Content: in.Draft}, nil
}

func startsWithHeading(s string) bool { return strings.HasPrefix(strings.TrimSpace(s), "#") }

// --- loop config (consumed by internal/build/loop) ---

func (l *LandingPage) DraftKey() string    { return keys.Draft }
func (l *LandingPage) CritiqueKey() string { return keys.Critique }

func (l *LandingPage) DrafterInstruction() string {
	return `You are the DRAFTER for a LANDING PAGE deliverable. Produce a Markdown
landing page with: a # hero headline, an explicit Offer line, three CTA variants,
and one A/B variant section. Use only the inputs below.`
}

func (l *LandingPage) CheckerInstruction() string {
	return `You are the CHECKER for a landing page. Judge ONLY whether the draft has:
(1) a hero headline, (2) an explicit offer, (3) at least one CTA, and is on-brand.
If it passes, call the exit_loop tool. If it fails, reply with JSON
{"critique": string, "fixes": [string]} and do NOT call any tool.`
}
