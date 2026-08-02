// internal/stages/research/research.go
package research

import (
	"context"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/genai"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
)

// New builds the research stage. A BeforeAgentCallback queries gBrain memory
// from the brief and injects results into the memory_context state key, so the
// model makes a single call (no tool round-trip → avoids the multi-turn bug).
func New(cfg stages.Config, b *brain.Brain) (agent.Agent, error) {
	mem := b.Memory()
	cb := func(ctx agent.Context) (*genai.Content, error) {
		// Skip if already populated (idempotent across re-runs).
		if v, _ := ctx.State().Get(keys.MemCtx); v != nil {
			return nil, nil
		}
		var query string
		if v, _ := ctx.State().Get(keys.Brief); v != nil {
			if s, ok := v.(string); ok {
				query = s
			}
		}
		// Use ctx.AppName()/ctx.UserID() directly: ctx.Session() returns nil
		// inside a BeforeAgentCallback (the callback context restricts it).
		found, err := brainLoad(ctx, mem, ctx.AppName(), ctx.UserID(), query)
		if err != nil {
			return nil, err
		}
		return nil, ctx.State().Set(keys.MemCtx, found)
	}

	return llmagent.New(llmagent.Config{
		Name:                 "research",
		Model:                cfg.Model,
		Description:          "Pulls relevant gBrain memory and assembles research findings.",
		IncludeContents:      llmagent.IncludeContentsNone,
		OutputKey:            keys.Research,
		BeforeAgentCallbacks: []agent.BeforeAgentCallback{cb},
		Instruction: cfg.Brand + `
You are the RESEARCH stage of a marketing engine.
The brief (JSON): {brief_output}

Relevant past memory (may be empty): {memory_context?}

Return JSON with EXACTLY: {"findings": [string], "relevant_rules": [string]}.
Return ONLY the JSON, no prose.`,
	})
}

// brainLoad queries gBrain memory scoped to the session's app/user.
func brainLoad(ctx context.Context, mem memory.Service, appName, userID, query string) (string, error) {
	if mem == nil || query == "" {
		return "", nil
	}
	resp, err := mem.SearchMemory(ctx, &memory.SearchRequest{Query: query, AppName: appName, UserID: userID})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, m := range resp.Memories {
		if m.Content == nil {
			continue
		}
		for _, p := range m.Content.Parts {
			if p.Text != "" {
				sb.WriteString(p.Text)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String(), nil
}
