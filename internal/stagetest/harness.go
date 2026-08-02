// Package stagetest provides fakes and a runner harness for stage unit tests.
// It is test-only (name encodes the convention); do not import from non-test code.
package stagetest

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// CannedLLM is a model.LLM that returns the same text on every call.
type CannedLLM struct{ Text string }

func (m *CannedLLM) Name() string { return "canned" }
func (m *CannedLLM) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{Content: genai.NewContentFromText(m.Text, "model")}, nil)
	}
}

var _ model.LLM = (*CannedLLM)(nil)

// ScriptedLLM returns Responses[i] on the (i+1)th call, cycling. If an entry
// starts with "FC:" the model instead emits a FunctionCall to that tool name.
type ScriptedLLM struct {
	Responses []string
	i         int
}

func (m *ScriptedLLM) Name() string { return "scripted" }
func (m *ScriptedLLM) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		raw := m.Responses[m.i%len(m.Responses)]
		m.i++
		resp := &model.LLMResponse{Content: &genai.Content{Role: "model"}}
		if len(raw) > 3 && raw[:3] == "FC:" {
			resp.Content.Parts = []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: raw[3:]}}}
		} else {
			resp.Content.Parts = []*genai.Part{genai.NewPartFromText(raw)}
		}
		yield(resp, nil)
	}
}

var _ model.LLM = (*ScriptedLLM)(nil)

// RunAgent runs ag once in a fresh in-memory session, seeded with state, and
// returns the state accumulated from every event's StateDelta.
func RunAgent(ctx context.Context, t *testing.T, ag agent.Agent, userID, userMsg string, seed map[string]any) map[string]any {
	t.Helper()
	const app = "engine_test"
	ss := session.InMemoryService()
	created, err := ss.Create(ctx, &session.CreateRequest{AppName: app, UserID: userID})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess := created.Session
	if len(seed) > 0 {
		ev := session.NewEvent(ctx, "seed")
		ev.Author = "user"
		ev.Actions = session.EventActions{StateDelta: seed}
		if err := ss.AppendEvent(ctx, sess, ev); err != nil {
			t.Fatalf("seed state: %v", err)
		}
	}
	r, err := runner.New(runner.Config{AppName: app, Agent: ag, SessionService: ss})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	out := map[string]any{}
	for ev, err := range r.Run(ctx, userID, sess.ID(), genai.NewContentFromText(userMsg, genai.RoleUser), agent.RunConfig{}) {
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if ev != nil && len(ev.Actions.StateDelta) > 0 {
			for k, v := range ev.Actions.StateDelta {
				out[k] = v
			}
		}
	}
	return out
}

// StateString reads key from a RunAgent result as a string.
func StateString(t *testing.T, state map[string]any, key string) string {
	t.Helper()
	v, ok := state[key]
	if !ok {
		t.Fatalf("state key %q not written by agent", key)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("state %q = %T, want string", key, v)
	}
	return s
}
