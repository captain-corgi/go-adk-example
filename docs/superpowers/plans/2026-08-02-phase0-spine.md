# Phase 0 — The Spine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a genuinely working end-to-end Marketing Engine run — one raw idea in → one landing-page deliverable out — that exercises every architectural concept exactly once, on top of the existing hello-world scaffold.

**Architecture:** A `sequentialagent` pipeline of seven stage agents (brief → ideation → research → synthesis → signoff → build → results) that pass JSON through session **state keys**. The `build` stage is a `workflow` graph containing one deliverable node wrapped in a `loopagent` eval loop (drafter → checker). gBrain = frozen brand context (injected into every stage instruction) + an in-memory `memory.Service` (read at research, written at results). A `routing.Models` registry resolves role → `model.LLM` (Phase 0: one model, two aliases).

**Tech Stack:** Go 1.26.5 · `google.golang.org/adk/v2` v2.1.0 (adk-go) · `github.com/joho/godotenv` · OpenAI-compatible model via `model/openaimodel` · standard `testing`.

## Global Constraints

Copied verbatim from the [Phase 0 spec](../specs/2026-08-02-phase0-spine-design.md) and shared [contract](../specs/00-marketing-engine-overview.md); apply to every task.

- **Module:** `github.com/captain-corgi/go-adk-example`. All internal imports use that prefix (e.g. `.../internal/keys`).
- **Go 1.26.5** — modern stdlib is available; do not assume old limitations.
- **Go version toolchain:** run `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -w .`, `goimports -w .`, `golangci-lint run`, `go mod tidy` as appropriate. `.golangci.yml` already enables govet, staticcheck, errcheck, ineffassign, unused, gofmt, goimports.
- **History-less, state-passing agents (load-bearing):** EVERY `llmagent` in this project sets `IncludeContents: llmagent.IncludeContentsNone` and passes data ONLY through session state keys (`OutputKey` to write, `{key}`/`{key?}` templating to read). This makes the pipeline stateless-correct AND avoids the known v2.1.0 `openaimodel` multi-turn bug (assistant turns mis-encoded as `input_text` → HTTP 400 on turn 2+). Do NOT rely on `Mode: ModeSingleTurn` to suppress history inside a `loopagent`/`sequentialagent` — it does not.
- **State keys are underscored identifiers** (see Deviation D1). Templating `{key}` only substitutes `^[a-zA-Z_][a-zA-Z0-9_]*$`; dotted keys are NOT substituted.
- **No `OutputSchema`** on any agent. It disables tools, and ADK v2.1.0 does not parse the JSON anyway. Stages request JSON in prose; consumers treat `*._output` values as strings.
- **Secrets never hardcoded.** Keys live in gitignored `.env`, read via `os.Getenv`. The only new env vars: `MODEL_STRONG`, `MODEL_CHEAP`, `AUTO_APPROVE`, `MAX_LOOP_ITER`, `GBRAIN_DIR`.
- **No real API key is required for any test.** All tests use hand-rolled fake `model.LLM`s.
- **Conventional Commits** (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`). Commit on the current branch (`feature/test-something`); PRs target `develop`, not `main`.
- **Never skip hooks** (`--no-verify`) or bypass signing.

## Deviations from the spec (authoritative for implementation)

These supers the corresponding lines in the specs; they are forced by real ADK v2.1.0 behavior (verified against source).

- **D1 — State keys are underscored, not dotted.** Spec §3.2 / Phase 0 §2 use `brief.output`, `ideation.output`, etc. The `{key}` instruction templating only substitutes valid identifiers, so `brief.output` would be passed through as the literal `{brief.output}`. This plan uses `brief_output`, `ideation_output`, `research_output`, `plan_output`, `signoff_output`, `build_output`, `results_output` (loop-internal: `landing_draft`, `landing_critique`, `landing_verdict`, `memory_context`). State keys remain the only coupling between stages.
- **D2 — Frozen brand context is injected into each stage's `Instruction`, not via `{artifact.*}` templating.** The artifact service binds artifacts to app/user/session, which are not known at startup; pre-loading brand into artifacts before a session exists is fragile. Instead the `brain` package reads `brand/*.md` once at startup into a string, and each stage prepends it to its instruction. The artifact service is still exercised — for **deliverable output storage** (results stage). `{artifact.*}` input templating is deferred to Phase 3+.
- **D3 — Research reads memory via a `BeforeAgentCallback`, not a model-called tool.** A tool round-trip (model emits functionCall → tool runs → model re-replies) re-sends an assistant turn and risks the multi-turn bug. A callback runs `memory.Service.SearchMemory` and writes the result to the `memory_context` state key, which the instruction templates as `{memory_context?}` — one model call, bug-immune. The "tools" concept is still exercised by the eval loop's `exitlooptool`.
- **D4 — Loop convergence is detected via `exitlooptool` + an `AfterToolCallback`.** The stock `exitlooptool.New()` escalates (exiting the loop) but does not record why. The checker carries an `AfterToolCallback` that sets `landing_verdict = "pass"` when `exit_loop` fires; if `MaxIterations` is reached first, `landing_verdict` stays unset and `finalize` marks the draft `max_iter_reached`. This honors contract §3.4 (checker calls `exitlooptool`) while making convergence observable.

## File Structure

```text
brand/                                 # gBrain frozen source (seed files, committed)
  voice.md  offers.md  icp.md          # short Markdown; engine runs out of the box
.env.example                           # add MODEL_STRONG, MODEL_CHEAP, AUTO_APPROVE, MAX_LOOP_ITER, GBRAIN_DIR
cmd/engine/main.go                     # CREATE — assembles pipeline + runs launcher
internal/
  keys/keys.go                         # state-key constants (leaf package, no deps)
  routing/routing.go                   # Models registry: Strong()/Cheap()/For(role)
  config/config.go                     # env load + openaimodel construction + Models
  brain/brain.go                       # brand loader (BrandContext) + memory.Service + Load (SearchMemory)
  stages/
    stage.go                           # shared stages.Config{Model,Brand}
    brief/brief.go                     # llmagent stage
    ideation/ideation.go               # llmagent stage
    research/research.go               # llmagent stage + memory BeforeAgentCallback
    synthesis/synthesis.go             # llmagent stage
    signoff/signoff.go                 # workflowagent HITL gate (AUTO_APPROVE + request_input)
    results/results.go                 # workflowagent: artifact save + AddSessionToMemory seam
  build/
    deliverables/deliverable.go        # Deliverable interface, BuildInput, Artifact
    deliverables/landingpage/landingpage.go  # LandingPage: Build() + loop-config accessors
    loop/loop.go                       # loopagent over drafter/checker + exitlooptool + verdict callback
    graph/graph.go                     # workflowagent: Start→loopNode→join→finalize
  stagetest/harness.go                 # test-only: fake models + RunAgent state harness
docs/superpowers/plans/2026-08-02-phase0-spine.md   # THIS FILE
```

**Import graph (no cycles):** `keys` (leaf) ← everything. `routing` ← `config`, `loop`, stages, `main`. `brain` ← stages/research, `main`. `deliverables` ← `landingpage`, `graph`. `loop` ← `graph`. `graph` ← `main`. `stagetest` ← all `*_test.go`.

---

## Task 1: State-key constants + routing registry

**Files:**
- Create: `internal/keys/keys.go`
- Create: `internal/routing/routing.go`
- Test: `internal/routing/routing_test.go`

**Interfaces:**
- Consumes: `google.golang.org/adk/v2/model` (`model.LLM`).
- Produces: `keys.Brief`…`keys.Results`, `keys.Draft`, `keys.Critique`, `keys.Verdict`, `keys.MemCtx`; `routing.Models` with `Strong()`, `Cheap()`, `For(role Role) model.LLM`, and constructor `routing.NewModels(strong, cheap model.LLM) Models`; `routing.Role`, `routing.StrongRole`, `routing.CheapRole`.

- [ ] **Step 1: Write the failing test**

```go
// internal/routing/routing_test.go
package routing_test

import (
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/routing/...`
Expected: FAIL — `routing: package not found` / `undefined: routing.NewModels`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/keys/keys.go
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
```

```go
// internal/routing/routing.go
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
```

Add the missing imports to the test file header (`context`, `iter`, `google.golang.org/genai` for the `model.LLM` method signatures). The stub's `GenerateContent` must match the interface exactly:

```go
import (
	"context"
	"iter"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/routing"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

func (s *stubLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(func(*model.LLMResponse, error) bool) {}
}
var _ model.LLM = (*stubLLM)(nil)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/routing/... ./internal/keys/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/keys internal/routing
git commit -m "feat: add state-key constants and routing.Models role registry"
```

---

## Task 2: Config (env + model construction)

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: `os`, `github.com/joho/godotenv`, `model/openaimodel`, `routing`.
- Produces: `config.Load() (*Config, error)` where `Config` holds `Models routing.Models`, `AutoApprove bool`, `MaxLoopIter int`, `GBrainDir string`. `config.Load` constructs one `openaimodel` from `MODEL_STRONG`/`MODEL_CHEAP` (both default to `OPENAI_MODEL`) and wraps both in `routing.NewModels`.

- [ ] **Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config_test

import (
	"os"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/config"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	// Simulate a minimal environment. A real API key is NOT required: the
	// openaimodel constructor only stores config; it does not dial out.
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("OPENAI_BASE_URL", "http://localhost")
	os.Setenv("OPENAI_MODEL", "gpt-test")
	os.Setenv("MODEL_STRONG", "gpt-strong")
	os.Setenv("MODEL_CHEAP", "gpt-cheap")
	os.Setenv("AUTO_APPROVE", "true")
	os.Setenv("MAX_LOOP_ITER", "5")
	os.Setenv("GBRAIN_DIR", "./brand")
	t.Cleanup(func() {
		os.Unsetenv("MODEL_STRONG"); os.Unsetenv("MODEL_CHEAP")
		os.Unsetenv("AUTO_APPROVE"); os.Unsetenv("MAX_LOOP_ITER"); os.Unsetenv("GBRAIN_DIR")
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.AutoApprove {
		t.Error("AutoApprove must be true when AUTO_APPROVE=true")
	}
	if cfg.MaxLoopIter != 5 {
		t.Errorf("MaxLoopIter = %d, want 5", cfg.MaxLoopIter)
	}
	if cfg.GBrainDir != "./brand" {
		t.Errorf("GBrainDir = %q, want ./brand", cfg.GBrainDir)
	}
	// Both roles resolve to non-nil models built from the named env models.
	if cfg.Models.Strong() == nil || cfg.Models.Cheap() == nil {
		t.Error("Models.Strong/Cheap must be non-nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/...`
Expected: FAIL — `undefined: config.Load`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/config/config.go
// Package config loads environment, builds the OpenAI-compatible model(s),
// and exposes the routing.Models registry plus engine knobs.
package config

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	openaimodel "google.golang.org/adk/v2/model/openaimodel"
)

const defaultModel = "gpt-5.6-sol"

// Config holds everything main.go needs to assemble the engine.
type Config struct {
	Models     routing.Models
	AutoApprove bool
	MaxLoopIter int
	GBrainDir   string
}

// Load reads .env (optional) + the process environment and builds the models.
func Load() (*Config, error) {
	// Missing .env is fine; malformed .env is not.
	if err := godotenv.Load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load .env: %w", err)
		}
		log.Printf("No .env file (%v); relying on process environment", err)
	}

	ctx := context.Background()
	strong := buildModel(ctx, firstNonEmpty(os.Getenv("MODEL_STRONG"), os.Getenv("OPENAI_MODEL")))
	cheap := buildModel(ctx, firstNonEmpty(os.Getenv("MODEL_CHEAP"), os.Getenv("OPENAI_MODEL")))

	return &Config{
		Models:     routing.NewModels(strong, cheap),
		AutoApprove: boolEnv("AUTO_APPROVE"),
		MaxLoopIter: intEnv("MAX_LOOP_ITER", 3),
		GBrainDir:   firstNonEmpty(os.Getenv("GBRAIN_DIR"), "./brand"),
	}, nil
}

func buildModel(ctx context.Context, name string) routing.Models {
	// Intentionally panics on failure via mustModel: the engine cannot run
	// without a model. Tests inject fakes, so they never call buildModel.
	panic("unreachable") // placeholder removed below
}
```

`buildModel` returning `routing.Models` is wrong — fix it to return `model.LLM` and drop the panic:

```go
import "google.golang.org/adk/v2/model"

func buildModel(ctx context.Context, name string) model.LLM {
	if name == "" {
		name = defaultModel
	}
	m, err := openaimodel.NewModel(ctx, name, &openaimodel.ClientConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
	})
	if err != nil {
		log.Fatalf("create model %q: %v", name, err)
	}
	return m
}
```

Helpers:

```go
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
func boolEnv(key string) bool { return os.Getenv(key) == "true" || os.Getenv(key) == "1" }
func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/config
git commit -m "feat: add config loader with env-driven model construction and routing"
```

---

## Task 3: Test harness (fake models + RunAgent)

**Files:**
- Create: `internal/stagetest/harness.go`
- Test: `internal/stagetest/harness_test.go`

**Interfaces:**
- Consumes: `model`, `genai`, `runner`, `session`, `agent`.
- Produces: `stagetest.CannedLLM` (returns one fixed text every call), `stagetest.ScriptedLLM` (returns `Responses[i]` cyclically, can emit a `FunctionCall`), `stagetest.RunAgent(ctx, t, agent, userID, userMsg, seed) map[string]any` (runs the agent in a fresh in-memory session seeded with state, returns the accumulated state), `stagetest.StateString(t, state, key)`.

- [ ] **Step 1: Write the failing test**

```go
// internal/stagetest/harness_test.go
package stagetest_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
)

func TestRunAgentCapturesOutputKey(t *testing.T) {
	ctx := context.Background()
	// A trivial agent that echoes and saves to a state key.
	ag, err := llmagent.New(llmagent.Config{
		Name:            "echo",
		Model:           &stagetest.CannedLLM{Text: "hello-out"},
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Brief,
		Instruction:     "Echo back whatever is asked.",
	})
	if err != nil {
		t.Fatal(err)
	}

	state := stagetest.RunAgent(ctx, t, ag, "u1", "ping", nil)
	if got := stagetest.StateString(t, state, keys.Brief); got != "hello-out" {
		t.Errorf("brief = %q, want hello-out", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stagetest/...`
Expected: FAIL — `undefined: stagetest.RunAgent`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/stagetest/harness.go
// Package stagetest provides fakes and a runner harness for stage unit tests.
// It is test-only (name encodes the convention); do not import from non-test code.
package stagetest

import (
	"context"
	"errors"
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
		ev.Actions = &session.EventActions{StateDelta: seed}
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
		if ev != nil && ev.Actions != nil {
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

// guard against unused import if errors/iter get dropped during edits
var _ = errors.Is
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/stagetest/...`
Expected: PASS. (If this fails on the harness mechanics, fix it here before later tasks depend on it.)

- [ ] **Step 5: Commit**

```sh
git add internal/stagetest
git commit -m "feat: add stage test harness with fake models and RunAgent"
```

---

## Task 4: brief stage

**Files:**
- Create: `internal/stages/stage.go`
- Create: `internal/stages/brief/brief.go`
- Test: `internal/stages/brief/brief_test.go`

**Interfaces:**
- Consumes: `keys`, `llmagent`, `model`; the `stagetest` harness in tests.
- Produces: `stages.Config{Model model.LLM; Brand string}`; `brief.New(cfg stages.Config) (agent.Agent, error)` — an llmagent that reads the user's idea (the turn content) and writes `keys.Brief`.

- [ ] **Step 1: Write the failing test**

```go
// internal/stages/brief/brief_test.go
package brief_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestBriefWritesStructuredJSON(t *testing.T) {
	canned := `{"product":"Widget","goal":"launch","audience":"builders","tone":"bold","constraints":[]}`
	ag, err := brief.New(stages.Config{
		Model: &stagetest.CannedLLM{Text: canned},
		Brand: "BRAND=acme\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1",
		"A no-code analytics tool for indie hackers", nil)

	out := stagetest.StateString(t, state, keys.Brief)
	var b map[string]any
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		t.Fatalf("brief_output not valid JSON: %v\nraw=%s", err, out)
	}
	if b["product"] != "Widget" {
		t.Errorf("product = %v, want Widget", b["product"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stages/brief/...`
Expected: FAIL — `undefined: brief.New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/stages/stage.go
// Package stages holds shared types for pipeline stage constructors.
package stages

import "google.golang.org/adk/v2/model"

// Config is the common input to every stage constructor.
type Config struct {
	Model model.LLM
	Brand string // frozen gBrain context, prepended to the Instruction
}
```

```go
// internal/stages/brief/brief.go
package brief

import (
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
)

// New builds the brief stage: turns the user's raw idea into a structured brief.
func New(cfg stages.Config) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:            "brief",
		Model:           cfg.Model,
		Description:     "Converts a raw marketing idea into a structured campaign brief.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Brief,
		Instruction: cfg.Brand + `
You are the BRIEF stage of a marketing engine. The user's message is a raw marketing idea.
Convert it into a concise structured brief as JSON with EXACTLY these fields:
{"product": string, "goal": string, "audience": string, "tone": string, "constraints": [string]}.
Return ONLY the JSON object, no prose, no code fences.`,
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/stages/brief/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/stages/stage.go internal/stages/brief
git commit -m "feat: add brief pipeline stage"
```

---

## Task 5: ideation stage

**Files:**
- Create: `internal/stages/ideation/ideation.go`
- Test: `internal/stages/ideation/ideation_test.go`

**Interfaces:**
- Consumes: `keys`, `stages.Config`, `llmagent`; reads `{brief_output}` via templating.
- Produces: `ideation.New(cfg stages.Config) (agent.Agent, error)` — writes `keys.Ideation`.

- [ ] **Step 1: Write the failing test**

```go
// internal/stages/ideation/ideation_test.go
package ideation_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestIdeationReadsBriefWritesAngles(t *testing.T) {
	canned := `{"angles":[{"name":"a","score":9,"rationale":"x"}]}`
	ag, err := ideation.New(stages.Config{Model: &stagetest.CannedLLM{Text: canned}, Brand: "BRAND=acme\n"})
	if err != nil {
		t.Fatal(err)
	}
	seed := map[string]any{keys.Brief: `{"product":"Widget","goal":"launch","audience":"builders","tone":"bold","constraints":[]}`}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go", seed)

	out := stagetest.StateString(t, state, keys.Ideation)
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("ideation_output not JSON: %v\n%s", err, out)
	}
	if _, ok := v["angles"]; !ok {
		t.Errorf("ideation_output missing 'angles': %s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stages/ideation/...`
Expected: FAIL — `undefined: ideation.New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/stages/ideation/ideation.go
package ideation

import (
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
)

// New builds the ideation stage: generates angles from the brief, scored
// against brand voice/ICP, keeping the top few.
func New(cfg stages.Config) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:            "ideation",
		Model:           cfg.Model,
		Description:     "Generates and scores campaign angles against the brief and brand.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Ideation,
		Instruction: cfg.Brand + `
You are the IDEATION stage of a marketing engine.
The structured brief (JSON):
{brief_output}

Generate roughly six campaign angles, score each (1-10) against the brand voice and the
brief's audience, and keep the top 2-3. Return JSON with EXACTLY:
{"angles": [{"name": string, "score": number, "rationale": string}]}.
Return ONLY the JSON, no prose.`,
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/stages/ideation/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/stages/ideation
git commit -m "feat: add ideation pipeline stage"
```

---

## Task 6: gBrain (brain package)

**Files:**
- Create: `brand/voice.md`, `brand/offers.md`, `brand/icp.md`
- Create: `internal/brain/brain.go`
- Test: `internal/brain/brain_test.go`

**Interfaces:**
- Consumes: `os`, `path/filepath`, `memory`, `genai`, `session`.
- Produces: `brain.New(dir string, mem memory.Service) (*Brain, error)`; `(*Brain).BrandContext() string` (concatenation of `*.md` under dir); `(*Brain).Memory() memory.Service`; `(*Brain).Load(ctx, appName, userID, query string) (string, error)` (runs `SearchMemory`, returns joined text or `""`).

- [ ] **Step 1: Write the failing test**

```go
// internal/brain/brain_test.go
package brain_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func TestBrandContextConcatenatesMarkdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "voice.md"), []byte("voice: bold"), 0o644)
	os.WriteFile(filepath.Join(dir, "offers.md"), []byte("offers: x"), 0o644)

	b, err := brain.New(dir, memory.InMemoryService())
	if err != nil {
		t.Fatal(err)
	}
	ctx := b.BrandContext()
	if !strings.Contains(ctx, "voice: bold") || !strings.Contains(ctx, "offers: x") {
		t.Errorf("BrandContext missing files: %q", ctx)
	}
}

func TestLoadReturnsEmptyWhenMemoryEmpty(t *testing.T) {
	b, _ := brain.New(t.TempDir(), memory.InMemoryService())
	got, err := b.Load(context.Background(), "app", "u1", "anything")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("Load on empty memory = %q, want empty", got)
	}
}

func TestLoadReturnsSeededMemory(t *testing.T) {
	dir := t.TempDir()
	mem := memory.InMemoryService()
	// Seed memory by ingesting a session with one user message.
	ctx := context.Background()
	ss := session.InMemoryService()
	created, _ := ss.Create(ctx, &session.CreateRequest{AppName: "app", UserID: "u1"})
	sess := created.Session
	ev := session.NewEvent(ctx, "seed")
	ev.Author = "user"
	ev.LLMResponse = model.LLMResponse{Content: genai.NewContentFromText("Tokyo trip converted well", genai.RoleUser)}
	ss.AppendEvent(ctx, sess, ev)
	mem.AddSessionToMemory(ctx, sess)

	b, _ := brain.New(dir, mem)
	got, _ := b.Load(ctx, "app", "u1", "Tokyo")
	if !strings.Contains(got, "Tokyo") {
		t.Errorf("Load missed seeded memory: %q", got)
	}
}
```

Add imports `strings` and `"google.golang.org/adk/v2/model"` to the test header.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/brain/...`
Expected: FAIL — `undefined: brain.New`.

- [ ] **Step 3: Write minimal implementation**

Seed brand files (short, so the engine runs out of the box):

```markdown
<!-- brand/voice.md -->
# Brand voice
Bold, concrete, no hype. Short sentences. Talk to builders, not executives.
```
```markdown
<!-- brand/offers.md -->
# Offers
Core offer: a no-code analytics tool. Pricing: free tier, then $29/mo.
```
```markdown
<!-- brand/icp.md -->
# Ideal customer profile
Indie hackers and small teams shipping side-projects who need product analytics
without setup overhead.
```

```go
// internal/brain/brain.go
// Package brain is gBrain: frozen brand context (read from disk) plus an
// in-memory memory.Service queried at research time.
package brain

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/adk/v2/memory"
)

type Brain struct {
	brand string
	mem   memory.Service
}

// New reads every *.md under dir (sorted by name) and concatenates them into
// the frozen brand context. Missing dir is non-fatal: BrandContext() is empty.
func New(dir string, mem memory.Service) (*Brain, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &Brain{mem: mem}, nil
		}
		return nil, fmt.Errorf("read brand dir %q: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", n, err)
		}
		sb.WriteString(string(b))
		sb.WriteString("\n")
	}
	return &Brain{brand: sb.String(), mem: mem}, nil
}

func (b *Brain) BrandContext() string     { return b.brand }
func (b *Brain) Memory() memory.Service   { return b.mem }

// Load queries gBrain memory and returns the concatenated matching text, or "".
func (b *Brain) Load(ctx context.Context, appName, userID, query string) (string, error) {
	if b.mem == nil {
		return "", nil
	}
	resp, err := b.mem.SearchMemory(ctx, &memory.SearchRequest{
		Query: query, AppName: appName, UserID: userID,
	})
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/brain/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add brand internal/brain
git commit -m "feat: add gBrain (brand context loader + memory.Service query)"
```

---

## Task 7: research stage (memory callback)

**Files:**
- Create: `internal/stages/research/research.go`
- Test: `internal/stages/research/research_test.go`

**Interfaces:**
- Consumes: `keys`, `stages`, `llmagent`, `brain`, `session`, `model`.
- Produces: `research.New(cfg stages.Config, mem memory.Service) (agent.Agent, error)`. The agent has a `BeforeAgentCallback` that reads `brief_output` from state, calls `brain.Load`-equivalent via the injected `memory.Service` using the session's AppName/UserID, and writes the result to `memory_context`. Its instruction reads `{brief_output}` + `{memory_context?}` and writes `keys.Research`.

- [ ] **Step 1: Write the failing test**

```go
// internal/stages/research/research_test.go
package research_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
	"google.golang.org/adk/v2/memory"
)

func TestResearchReadsBriefAndWritesOutput(t *testing.T) {
	b, err := brain.New(t.TempDir(), memory.InMemoryService())
	if err != nil {
		t.Fatal(err)
	}
	canned := `{"findings":["f1","f2"],"relevant_rules":[]}`
	ag, err := research.New(stages.Config{Model: &stagetest.CannedLLM{Text: canned}, Brand: ""}, b)
	if err != nil {
		t.Fatal(err)
	}
	seed := map[string]any{keys.Brief: `{"product":"Widget","goal":"launch","audience":"builders","tone":"bold","constraints":[]}`}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go", seed)

	out := stagetest.StateString(t, state, keys.Research)
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("research_output not JSON: %v\n%s", err, out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stages/research/...`
Expected: FAIL — `undefined: research.New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/stages/research/research.go
package research

import (
	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/memory"
)

// New builds the research stage. A BeforeAgentCallback queries gBrain memory
// from the brief and injects results into the memory_context state key, so the
// model makes a single call (no tool round-trip → avoids the multi-turn bug).
func New(cfg stages.Config, b *brain.Brain) (agent.Agent, error) {
	mem := b.Memory()
	cb := func(ctx agent.Context) (*session.Event, error) {
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
		sess := ctx.Session()
		found, err := brainLoad(ctx, mem, sess.AppName(), sess.UserID(), query)
		if err != nil {
			return nil, err
		}
		return nil, ctx.State().Set(keys.MemCtx, found)
	}

	return llmagent.New(llmagent.Config{
		Name:            "research",
		Model:           cfg.Model,
		Description:     "Pulls relevant gBrain memory and assembles research findings.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Research,
		BeforeAgentCallbacks: []agent.BeforeAgentCallback{cb},
		Instruction: cfg.Brand + `
You are the RESEARCH stage of a marketing engine.
The brief (JSON): {brief_output}

Relevant past memory (may be empty): {memory_context?}

Return JSON with EXACTLY: {"findings": [string], "relevant_rules": [string]}.
Return ONLY the JSON, no prose.`,
	})
}

// brainLoad is a tiny indirection so tests could swap behavior; it delegates to
// the shared brain package's memory query.
func brainLoad(ctx context.Context, mem memory.Service, appName, userID, query string) (string, error) {
	if mem == nil || query == "" {
		return "", nil
	}
	resp, err := mem.SearchMemory(ctx, &memory.SearchRequest{Query: query, AppName: appName, UserID: userID})
	if err != nil {
		return "", err
	}
	out := ""
	for _, m := range resp.Memories {
		if m.Content == nil {
			continue
		}
		for _, p := range m.Content.Parts {
			if p.Text != "" {
				out += p.Text + "\n"
			}
		}
	}
	return out, nil
}
```

Add `"context"` and `"google.golang.org/adk/v2/session"` to the imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/stages/research/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/stages/research
git commit -m "feat: add research stage with gBrain memory callback"
```

---

## Task 8: synthesis stage

**Files:**
- Create: `internal/stages/synthesis/synthesis.go`
- Test: `internal/stages/synthesis/synthesis_test.go`

**Interfaces:**
- Consumes: `keys`, `stages`, `llmagent`; reads `{brief_output}`, `{ideation_output}`, `{research_output}`.
- Produces: `synthesis.New(cfg stages.Config) (agent.Agent, error)` — writes `keys.Plan`.

- [ ] **Step 1: Write the failing test**

```go
// internal/stages/synthesis/synthesis_test.go
package synthesis_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestSynthesisMergesInputsIntoPlan(t *testing.T) {
	canned := `{"hook":"h","offer":"o","message_hierarchy":["a"],"cta":"Sign up","deliverable":"landing_page"}`
	ag, err := synthesis.New(stages.Config{Model: &stagetest.CannedLLM{Text: canned}, Brand: ""})
	if err != nil {
		t.Fatal(err)
	}
	seed := map[string]any{
		keys.Brief:    `{"product":"Widget"}`,
		keys.Ideation: `{"angles":[]}`,
		keys.Research: `{"findings":[]}`,
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go", seed)

	out := stagetest.StateString(t, state, keys.Plan)
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("plan_output not JSON: %v\n%s", err, out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/stages/synthesis/...`
Expected: FAIL — `undefined: synthesis.New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/stages/synthesis/synthesis.go
package synthesis

import (
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
)

// New builds the synthesis stage: merges brief, angles, and research into a
// concrete campaign plan.
func New(cfg stages.Config) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:            "synthesis",
		Model:           cfg.Model,
		Description:     "Merges brief, ideation, and research into a campaign plan.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       keys.Plan,
		Instruction: cfg.Brand + `
You are the SYNTHESIS stage of a marketing engine.
Brief (JSON): {brief_output}
Angles (JSON): {ideation_output}
Research (JSON): {research_output}

Produce ONE concrete campaign plan as JSON with EXACTLY:
{"hook": string, "offer": string, "message_hierarchy": [string], "cta": string, "deliverable": "landing_page"}.
Return ONLY the JSON, no prose.`,
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/stages/synthesis/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/stages/synthesis
git commit -m "feat: add synthesis pipeline stage"
```

---

## Task 9: Deliverable interface + landing page

**Files:**
- Create: `internal/build/deliverables/deliverable.go`
- Create: `internal/build/deliverables/landingpage/landingpage.go`
- Test: `internal/build/deliverables/landingpage/landingpage_test.go`

**Interfaces:**
- Consumes: `encoding/json`.
- Produces: `deliverables.BuildInput{Brief, Plan, Research, Brand json.RawMessage; Draft string}`, `deliverables.Artifact{Name, Content string}`, `deliverables.Deliverable` interface (`Name() string; Build(ctx, BuildInput) (*Artifact, error)`); `landingpage.New() *LandingPage`; `(*LandingPage).Name()/"landing_page"`, `Build(ctx, BuildInput) (*Artifact, error)` (validates the draft has a hero, an offer, and ≥1 CTA; returns the artifact or an error), plus loop-config accessors `DrafterInstruction()`, `CheckerInstruction()`, `DraftKey()`, `CritiqueKey()`.

- [ ] **Step 1: Write the failing test**

```go
// internal/build/deliverables/landingpage/landingpage_test.go
package landingpage_test

import (
	"context"
	"errors"
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
	// Must NOT be a non-meaningful error type: just confirm it is non-nil.
	_ = errors.Is
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/build/deliverables/landingpage/...`
Expected: FAIL — `undefined: landingpage.New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/build/deliverables/deliverable.go
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
```

```go
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
	if !strings.Contains(d, "cta") && !strings.Contains(strings.ToLower(in.Draft), "sign up") && !strings.Contains(d, "start") {
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/build/deliverables/landingpage/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/build/deliverables
git commit -m "feat: add Deliverable interface and landing-page vertical"
```

---

## Task 10: Eval loop (loopagent + exitlooptool + verdict)

**Files:**
- Create: `internal/build/loop/loop.go`
- Test: `internal/build/loop/loop_test.go`

**Interfaces:**
- Consumes: `keys`, `routing`, `llmagent`, `loopagent`, `exitlooptool`, `tool`, the landing page's loop-config accessors (via a local `LoopSpec` interface so the loop is generic).
- Produces: `loop.LoopSpec` interface (`DrafterInstruction()`, `CheckerInstruction()`, `DraftKey()`, `CritiqueKey() string`); `loop.New(spec LoopSpec, models routing.Models, maxIter int) (agent.Agent, error)`. Builds a `loopagent` over `[drafter, checker]`. The drafter uses `models.Cheap()`, `IncludeContentsNone`, `OutputKey=DraftKey`, instruction templates `{plan_output}` + `{<critiquekey>?}`. The checker uses `models.Strong()`, `IncludeContentsNone`, `OutputKey=CritiqueKey`, `Tools=[exitlooptool]`, and an `AfterToolCallback` setting `keys.Verdict="pass"` when `exit_loop` fires.

- [ ] **Step 1: Write the failing test**

```go
// internal/build/loop/loop_test.go
package loop_test

import (
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/build/deliverables/landingpage"
	"github.com/captain-corgi/go-adk-example/internal/build/loop"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

// specStub lets the test supply loop config without depending on landingpage.
type specStub struct{ lp *landingpage.LandingPage }

func (s specStub) DrafterInstruction() string  { return s.lp.DrafterInstruction() }
func (s specStub) CheckerInstruction() string  { return s.lp.CheckerInstruction() }
func (s specStub) DraftKey() string            { return s.lp.DraftKey() }
func (s specStub) CritiqueKey() string         { return s.lp.CritiqueKey() }

func models(drafter, checker stagetest.CannedLike) routing.Models {
	return routing.NewModels(checker.AsLLM(), drafter.AsLLM()) // strong=checker, cheap=drafter
}

func TestLoopExitsWhenCheckerPasses(t *testing.T) {
	// Drafter always returns a valid draft; checker critiques once, then calls exit_loop.
	// We need scripted behavior, so use ScriptedLLM for the checker.
	drafterModel := &stagetest.CannedLLM{Text: "# Hero\nOffer: free\nCTA: Sign up"}
	checkerModel := &stagetest.ScriptedLLM{Responses: []string{
		`{"critique":"add cta","fixes":["cta"]}`, // iter 1: fail
		"FC:exit_loop",                            // iter 2: pass
	}}
	m := routing.NewModels(checkerModel, drafterModel)

	ag, err := loop.New(specStub{lp: landingpage.New()}, m, 5)
	if err != nil {
		t.Fatal(err)
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go",
		map[string]any{keys.Plan: `{"deliverable":"landing_page"}`})

	if stagetest.StateString(t, state, keys.Verdict) != "pass" {
		t.Error("checker pass must set landing_verdict=pass")
	}
	if stagetest.StateString(t, state, keys.Draft) == "" {
		t.Error("drafter must write landing_draft")
	}
}

func TestLoopMaxIterShipsBestDraft(t *testing.T) {
	// Checker never calls exit_loop -> MaxIterations -> verdict unset.
	drafterModel := &stagetest.CannedLLM{Text: "# Hero\nOffer: free\nCTA: Sign up"}
	checkerModel := &stagetest.CannedLLM{Text: `{"critique":"nope","fixes":[]}`}
	m := routing.NewModels(checkerModel, drafterModel)

	ag, err := loop.New(specStub{lp: landingpage.New()}, m, 2)
	if err != nil {
		t.Fatal(err)
	}
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "go",
		map[string]any{keys.Plan: `{}`})

	if stagetest.StateString(t, state, keys.Draft) == "" {
		t.Error("best draft must still be written at max_iter")
	}
	if _, ok := state[keys.Verdict]; ok {
		t.Error("verdict must stay unset when the loop never converges")
	}
}
```

(The `stagetest.CannedLike`/`AsLLM` helper above is illustrative — replace it by passing the concrete `*stagetest.CannedLLM` / `*stagetest.ScriptedLLM` directly to `routing.NewModels`, since both already satisfy `model.LLM`. Drop the `models(...)` helper and the `CannedLike` type; call `routing.NewModels(checkerModel, drafterModel)` inline.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/build/loop/...`
Expected: FAIL — `undefined: loop.New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/build/loop/loop.go
package loop

import (
	"fmt"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/agent/workflowagents/loopagent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/exitlooptool"
)

// LoopSpec is the per-vertical config the loop needs to build its agents.
// *landingpage.LandingPage satisfies it; Phase 1 formalizes this as Loopable.
type LoopSpec interface {
	DrafterInstruction() string
	CheckerInstruction() string
	DraftKey() string
	CritiqueKey() string
}

// New builds a loopagent over [drafter, checker]. The drafter produces the
// deliverable; the checker calls exitlooptool on pass. Both are history-less,
// state-passing agents (IncludeContentsNone) to avoid the openaimodel
// multi-turn bug.
func New(spec LoopSpec, models routing.Models, maxIter int) (agent.Agent, error) {
	if maxIter < 1 {
		maxIter = 1
	}
	drafter, err := llmagent.New(llmagent.Config{
		Name:            "drafter",
		Model:           models.Cheap(),
		Description:     "Produces a deliverable draft, improved from prior critique.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       spec.DraftKey(),
		Instruction: fmt.Sprintf(`%s

Campaign plan (JSON):
{plan_output}

Prior critique, if any:
{%s?}

Return ONLY the deliverable Markdown.`, spec.DrafterInstruction(), spec.CritiqueKey()),
	})
	if err != nil {
		return nil, fmt.Errorf("drafter: %w", err)
	}

	exitTool, err := exitlooptool.New()
	if err != nil {
		return nil, fmt.Errorf("exitlooptool: %w", err)
	}

	checker, err := llmagent.New(llmagent.Config{
		Name:            "checker",
		Model:           models.Strong(),
		Description:     "Reviews the draft; calls exit_loop when it passes.",
		IncludeContents: llmagent.IncludeContentsNone,
		OutputKey:       spec.CritiqueKey(),
		Tools:           []tool.Tool{exitTool},
		AfterToolCallbacks: []llmagent.AfterToolCallback{verdictOnExit},
		Instruction: fmt.Sprintf(`%s

Draft to review:
{%s}`, spec.CheckerInstruction(), spec.DraftKey()),
	})
	if err != nil {
		return nil, fmt.Errorf("checker: %w", err)
	}

	return loopagent.New(loopagent.Config{
		MaxIterations: uint(maxIter),
		AgentConfig: agent.Config{
			Name:      "eval_loop",
			SubAgents: []agent.Agent{drafter, checker},
		},
	})
}

// verdictOnExit stamps landing_verdict=pass when the exit_loop tool runs.
func verdictOnExit(ctx agent.Context, tl tool.Tool, _, result map[string]any, err error) (map[string]any, error) {
	if err == nil && tl != nil && tl.Name() == "exit_loop" {
		_ = ctx.State().Set(keys.Verdict, "pass")
	}
	return result, err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/build/loop/...`
Expected: PASS. Both `TestLoopExitsWhenCheckerPasses` (verdict=pass) and `TestLoopMaxIterShipsBestDraft` (draft present, verdict absent) must pass.

- [ ] **Step 5: Commit**

```sh
git add internal/build/loop
git commit -m "feat: add eval loop (loopagent + exitlooptool + verdict callback)"
```

---

## Task 11: Build graph (workflow: loop → join → finalize)

**Files:**
- Create: `internal/build/graph/graph.go`
- Test: `internal/build/graph/graph_test.go`

**Interfaces:**
- Consumes: `keys`, `routing`, `loop`, `landingpage`, `deliverables`, `workflow`, `workflowagent`, `session`, `genai`.
- Produces: `graph.New(models routing.Models, maxIter int) (agent.Agent, error)` — a `workflowagent` named "build" with edges `Start → loopNode → joinNode → finalizeNode` (loopNode = `NewAgentNode(loopAgent)`, joinNode = `NewJoinNode("build_join")`, finalizeNode = `NewFunctionNode` that reads `landing_draft` + `plan_output` from state, calls `landingpage.Build`, saves the artifact via `ctx.Artifacts().Save`, and writes `keys.Build`). Uses `AddFanOut`/`AddFanIn` so Phase 1 adds a second deliverable as a new edge, not a refactor.

- [ ] **Step 1: Write the failing test**

```go
// internal/build/graph/graph_test.go
package graph_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/build/graph"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
	"google.golang.org/adk/v2/artifact"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func TestBuildGraphProducesBuildOutput(t *testing.T) {
	// Drafter returns a valid landing page; checker never exits (max_iter).
	drafter := &stagetest.CannedLLM{Text: "# Hero\nOffer: free\nCTA: Sign up"}
	checker := &stagetest.CannedLLM{Text: `{"critique":"x","fixes":[]}`}
	m := routing.NewModels(checker, drafter)

	ag, err := graph.New(m, 2)
	if err != nil {
		t.Fatal(err)
	}

	// Run via the harness pattern but with an artifact service attached.
	const app = "engine_test"
	ctx := context.Background()
	ss := session.InMemoryService()
	as := artifact.InMemoryService()
	created, _ := ss.Create(ctx, &session.CreateRequest{AppName: app, UserID: "u1"})
	sess := created.Session
	// Seed the plan the drafter reads.
	sev := session.NewEvent(ctx, "seed")
	sev.Author = "user"
	sev.Actions = &session.EventActions{StateDelta: map[string]any{keys.Plan: `{"deliverable":"landing_page"}`}}
	ss.AppendEvent(ctx, sess, sev)

	r, err := runner.New(runner.Config{AppName: app, Agent: ag, SessionService: ss, ArtifactService: as})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	for ev, err := range r.Run(ctx, "u1", sess.ID(), genai.NewContentFromText("build", genai.RoleUser), agent.RunConfig{}) {
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if ev != nil && ev.Actions != nil {
			for k, v := range ev.Actions.StateDelta {
				got[k] = v
			}
		}
	}
	if _, ok := got[keys.Build]; !ok {
		t.Fatalf("build_output not written; state=%v", got)
	}
}
```

Add `"google.golang.org/adk/v2/agent"` to the test imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/build/graph/...`
Expected: FAIL — `undefined: graph.New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/build/graph/graph.go
package graph

import (
	"encoding/json"
	"fmt"

	"github.com/captain-corgi/go-adk-example/internal/build/deliverables"
	"github.com/captain-corgi/go-adk-example/internal/build/deliverables/landingpage"
	"github.com/captain-corgi/go-adk-example/internal/build/loop"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

// New builds the "build" stage: a workflow graph that runs the landing-page
// eval loop, then finalizes (validate + save artifact + write build_output).
// The fan-out/fan-in structure is in place even at N=1 so Phase 1 only adds edges.
func New(models routing.Models, maxIter int) (agent.Agent, error) {
	lp := landingpage.New()
	loopAgent, err := loop.New(lp, models, maxIter)
	if err != nil {
		return nil, fmt.Errorf("build loop: %w", err)
	}

	loopNode, err := workflow.NewAgentNode(loopAgent, workflow.NodeConfig{})
	if err != nil {
		return nil, fmt.Errorf("loop node: %w", err)
	}
	joinNode := workflow.NewJoinNode("build_join")
	finalizeNode := workflow.NewFunctionNode("finalize", finalize(lp), workflow.NodeConfig{})

	eb := workflow.NewEdgeBuilder()
	eb.AddFanOut(workflow.Start, loopNode)
	eb.AddFanIn(joinNode, loopNode)
	eb.Add(joinNode, finalizeNode)

	return workflowagent.New(workflowagent.Config{
		Name:        "build",
		Description: "Runs the deliverable eval loop and finalizes artifacts.",
		Edges:       eb.Build(),
		SubAgents:   []agent.Agent{loopAgent},
	})
}

// finalize returns the function-node body: read the settled draft + plan from
// state, validate via the deliverable, save the artifact, and write build_output.
func finalize(lp *landingpage.LandingPage) func(ctx agent.Context, _ map[string]any) (map[string]any, error) {
	return func(ctx agent.Context, _ map[string]any) (map[string]any, error) {
		draftStr := stateString(ctx, keys.Draft)
		planStr := stateString(ctx, keys.Plan)

		verdict := stateString(ctx, keys.Verdict)
		quality := "max_iter_reached"
		if verdict == "pass" {
			quality = "approved"
		}

		art, err := lp.Build(ctx, deliverables.BuildInput{
			Draft: draftStr,
			Plan:  json.RawMessage(planStr),
		})
		if err != nil {
			// Non-converged drafts may fail validation; still ship what we have.
			art = &deliverables.Artifact{Name: "landing_page.md", Content: draftStr}
		}
		if _, err := ctx.Artifacts().Save(ctx, art.Name, genai.NewPartFromText(art.Content, "text/plain")); err != nil {
			return nil, fmt.Errorf("save artifact: %w", err)
		}

		out := map[string]any{"landing_page": art.Name, "quality": quality}
		payload, _ := json.Marshal(out)
		if err := ctx.State().Set(keys.Build, string(payload)); err != nil {
			return nil, err
		}
		return out, nil
	}
}

func stateString(ctx agent.Context, key string) string {
	v, _ := ctx.State().Get(key)
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

var _ = session.NewEvent // referenced for docs; remove if unused by linter
```

If the linter flags the `session` import as unused, drop that import and the `var _ =` line.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/build/graph/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/build/graph
git commit -m "feat: add build workflow graph (loop → join → finalize)"
```

---

## Task 12: signoff stage (HITL gate) + results stage (memory seam)

**Files:**
- Create: `internal/stages/signoff/signoff.go`
- Create: `internal/stages/results/results.go`
- Test: `internal/stages/signoff/signoff_test.go`, `internal/stages/results/results_test.go`

**Interfaces:**
- Consumes: `keys`, `workflow`, `workflowagent`, `session`, `memory`, `brain`.
- Produces: `signoff.New(autoApprove bool) (agent.Agent, error)` (a workflowagent; AUTO_APPROVE copies `plan_output`→`signoff_output`; otherwise uses `workflow.ResumeOrRequestInput` for approve/edit). `results.New(mem memory.Service) (agent.Agent, error)` (a workflowagent whose node calls `mem.AddSessionToMemory(ctx, ctx.Session())` and writes `keys.Results`).

- [ ] **Step 1: Write the failing tests**

```go
// internal/stages/signoff/signoff_test.go
package signoff_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages/signoff"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
)

func TestAutoApproveCopiesPlanToSignoff(t *testing.T) {
	ag, err := signoff.New(true)
	if err != nil {
		t.Fatal(err)
	}
	plan := `{"hook":"h","offer":"o","cta":"Sign up","deliverable":"landing_page"}`
	state := stagetest.RunAgent(context.Background(), t, ag, "u1", "approve",
		map[string]any{keys.Plan: plan})

	if got := stagetest.StateString(t, state, keys.Signoff); got != plan {
		t.Errorf("signoff_output = %q, want the plan unchanged under AUTO_APPROVE", got)
	}
}
```

```go
// internal/stages/results/results_test.go
package results_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/stages/results"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
	"google.golang.org/adk/v2/memory"
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/stages/signoff/... ./internal/stages/results/...`
Expected: FAIL — `undefined: signoff.New`, `undefined: results.New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/stages/signoff/signoff.go
package signoff

import (
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

// New builds the sign-off gate. With autoApprove it copies plan_output to
// signoff_output (logged via state). Otherwise it pauses for a human
// approve/edit decision via workflow HITL.
func New(autoApprove bool) (agent.Agent, error) {
	rerun := true
	node := workflow.NewEmittingFunctionNode[any, any]("signoff",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			plan := stateString(ctx, keys.Plan)

			if autoApprove {
				_ = ctx.State().Set(keys.Signoff, plan)
				return nil, nil
			}

			reply, err := workflow.ResumeOrRequestInput(ctx, emit, session.RequestInput{
				InterruptID: "signoff",
				Message:     "Approve this campaign plan? Reply 'approve' or paste an edited plan.",
			})
			if err != nil {
				return nil, err
			}
			decision, _ := reply.(string)
			if decision == "approve" || decision == "" {
				_ = ctx.State().Set(keys.Signoff, plan)
			} else {
				_ = ctx.State().Set(keys.Signoff, decision) // edited plan
			}
			return nil, nil
		}, workflow.NodeConfig{RerunOnResume: &rerun})

	eb := workflow.NewEdgeBuilder()
	eb.Add(workflow.Start, node)
	return workflowagent.New(workflowagent.Config{
		Name:        "signoff",
		Description: "Human sign-off gate between synthesis and build.",
		Edges:       eb.Build(),
	})
}

func stateString(ctx agent.Context, key string) string {
	v, _ := ctx.State().Get(key)
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
```

```go
// internal/stages/results/results.go
package results

import (
	"encoding/json"
	"log"

	"github.com/captain-corgi/go-adk-example/internal/keys"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/workflow"
)

// New builds the results stage: ingests the session into gBrain memory (the
// feedback-loop seam; enriched in Phase 3) and writes results_output.
func New(mem memory.Service) (agent.Agent, error) {
	node := workflow.NewFunctionNode[any, any]("record_results",
		func(ctx agent.Context, _ any) (any, error) {
			if mem != nil {
				if err := mem.AddSessionToMemory(ctx, ctx.Session()); err != nil {
					// Non-fatal: the feedback seam is best-effort in Phase 0.
					log.Printf("results: AddSessionToMemory failed: %v", err)
				}
			}
			out := map[string]any{"status": "complete"}
			payload, _ := json.Marshal(out)
			_ = ctx.State().Set(keys.Results, string(payload))
			return nil, nil
		}, workflow.NodeConfig{})

	eb := workflow.NewEdgeBuilder()
	eb.Add(workflow.Start, node)
	return workflowagent.New(workflowagent.Config{
		Name:        "results",
		Description: "Persists results and writes the gBrain memory feedback seam.",
		Edges:       eb.Build(),
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/stages/signoff/... ./internal/stages/results/...`
Expected: PASS. (The interactive HITL path is verified manually; the unit test covers the AUTO_APPROVE path used by the E2E.)

- [ ] **Step 5: Commit**

```sh
git add internal/stages/signoff internal/stages/results
git commit -m "feat: add signoff HITL gate and results memory-seam stage"
```

---

## Task 13: Pipeline assembly (cmd/engine/main.go) + contract test

**Files:**
- Create: `cmd/engine/main.go`
- Delete: `main.go` (the hello-world scaffold at the repo root)
- Test: `internal/contract/contract_test.go`

**Interfaces:**
- Consumes: everything built above + `sequentialagent`, `launcher`, `full`, `brain`, `memory`, `artifact`.
- Produces: a runnable `cmd/engine/main.go` that builds models, gBrain, stages, build graph, assembles them into one `sequentialagent`, and runs via `full.NewLauncher()` with `ArtifactService` + `MemoryService` wired. A contract test asserting every stage publishes/accepts its contracted state key.

- [ ] **Step 1: Write the failing test**

```go
// internal/contract/contract_test.go
package contract_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/build/graph"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
	"google.golang.org/adk/v2/memory"
)

// TestEachStagePublishesItsKey is the swappability contract: each stage, fed its
// input key, writes its output key. Guards that no stage silently changes names.
func TestEachStagePublishesItsKey(t *testing.T) {
	ctx := context.Background()
	canned := `{"ok":true}`
	m := routing.NewModels(&stagetest.CannedLLM{Text: canned}, &stagetest.CannedLLM{Text: canned})

	cases := []struct {
		name     string
		build    func() (any, error)
		seed     map[string]any
		wantKey  string
	}{
		{"brief", func() (any, error) { return brief.New(stages.Config{Model: m.Cheap(), Brand: ""}) }, nil, keys.Brief},
		{"ideation", func() (any, error) { return ideation.New(stages.Config{Model: m.Cheap(), Brand: ""}) }, map[string]any{keys.Brief: `{}`}, keys.Ideation},
		{"synthesis", func() (any, error) { return synthesis.New(stages.Config{Model: m.Strong(), Brand: ""}) }, map[string]any{keys.Brief: `{}`, keys.Ideation: `{}`, keys.Research: `{}`}, keys.Plan},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := c.build()
			if err != nil {
				t.Fatal(err)
			}
			ag, ok := a.(interface {
				Run(agent.Context) iter.Seq2[*session.Event, error]
			})
			_ = ag
			_ = ok
			// Use the harness which works on agent.Agent.
			agentVal := toAgent(t, a)
			state := stagetest.RunAgent(ctx, t, agentVal, "u1", "x", c.seed)
			if _, ok := state[c.wantKey]; !ok {
				t.Errorf("%s did not publish %s; state=%v", c.name, c.wantKey, state)
			}
		})
	}
	// research needs a brain; graph is exercised in its own package.
	b, _ := researchNewHelper(t)
	_ = b
}

// toAgent converts the any returned by a constructor to agent.Agent.
func toAgent(t *testing.T, a any) agent.Agent {
	t.Helper()
	ag, ok := a.(agent.Agent)
	if !ok {
		t.Fatalf("not an agent.Agent: %T", a)
	}
	return ag
}

func researchNewHelper(t *testing.T) (*research.Agent, error) {
	_ = t
	return nil, nil
}
```

The above has deliberate scaffolding noise (`iter`, `researchNewHelper`, `Agent`). Replace the whole file with this clean version before running:

```go
// internal/contract/contract_test.go
package contract_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
	"github.com/captain-corgi/go-adk-example/internal/stagetest"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/memory"
)

func TestEachStagePublishesItsKey(t *testing.T) {
	ctx := context.Background()
	m := routing.NewModels(&stagetest.CannedLLM{Text: `{"ok":true}`}, &stagetest.CannedLLM{Text: `{"ok":true}`})
	b, err := brain.New(t.TempDir(), memory.InMemoryService())
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		build   func() (agent.Agent, error)
		seed    map[string]any
		wantKey string
	}{
		{"brief", func() (agent.Agent, error) { return brief.New(stages.Config{Model: m.Cheap()}) }, nil, keys.Brief},
		{"ideation", func() (agent.Agent, error) { return ideation.New(stages.Config{Model: m.Cheap()}) }, map[string]any{keys.Brief: `{}`}, keys.Ideation},
		{"research", func() (agent.Agent, error) { return research.New(stages.Config{Model: m.Cheap()}, b) }, map[string]any{keys.Brief: `{}`}, keys.Research},
		{"synthesis", func() (agent.Agent, error) { return synthesis.New(stages.Config{Model: m.Strong()}) }, map[string]any{keys.Brief: `{}`, keys.Ideation: `{}`, keys.Research: `{}`}, keys.Plan},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ag, err := c.build()
			if err != nil {
				t.Fatal(err)
			}
			state := stagetest.RunAgent(ctx, t, ag, "u1", "x", c.seed)
			if _, ok := state[c.wantKey]; !ok {
				t.Errorf("%s did not publish %s; state=%v", c.name, c.wantKey, state)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/contract/...`
Expected: FAIL — package/test do not exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
// cmd/engine/main.go
// Command engine runs the Marketing Engine: one raw idea in, one landing-page
// deliverable out, end to end, on adk-go.
package main

import (
	"context"
	"log"
	"os"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/build/graph"
	"github.com/captain-corgi/go-adk-example/internal/config"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stages/results"
	"github.com/captain-corgi/go-adk-example/internal/stages/signoff"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/v2/artifact"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/memory"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()

	mem := memory.InMemoryService()
	art := artifact.InMemoryService()
	b, err := brain.New(cfg.GBrainDir, mem)
	if err != nil {
		log.Fatalf("brain: %v", err)
	}
	brand := b.BrandContext()

	briefAg, err := brief.New(stages.Config{Model: cfg.Models.Cheap(), Brand: brand})
	if err != nil {
		log.Fatal(err)
	}
	ideationAg, err := ideation.New(stages.Config{Model: cfg.Models.Cheap(), Brand: brand})
	if err != nil {
		log.Fatal(err)
	}
	researchAg, err := research.New(stages.Config{Model: cfg.Models.Cheap(), Brand: brand}, b)
	if err != nil {
		log.Fatal(err)
	}
	synthesisAg, err := synthesis.New(stages.Config{Model: cfg.Models.Strong(), Brand: brand})
	if err != nil {
		log.Fatal(err)
	}
	signoffAg, err := signoff.New(cfg.AutoApprove)
	if err != nil {
		log.Fatal(err)
	}
	buildAg, err := graph.New(cfg.Models, cfg.MaxLoopIter)
	if err != nil {
		log.Fatal(err)
	}
	resultsAg, err := results.New(mem)
	if err != nil {
		log.Fatal(err)
	}

	pipeline, err := sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name: "marketing_engine",
			SubAgents: []agent.Agent{
				briefAg, ideationAg, researchAg, synthesisAg, signoffAg, buildAg, resultsAg,
			},
			Description: "Raw marketing idea in, shipped landing-page campaign out.",
		},
	})
	if err != nil {
		log.Fatalf("pipeline: %v", err)
	}

	launcherCfg := &launcher.Config{
		AgentLoader:     agent.NewSingleLoader(pipeline),
		ArtifactService: art,
		MemoryService:   mem,
	}

	l := full.NewLauncher()
	if err := l.Execute(ctx, launcherCfg, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
```

Delete the old scaffold so the module has a single entry point:

```sh
git rm main.go
```

If `go mod tidy` is needed after the new imports, run it (Task 15 covers a final tidy, but a preliminary one here avoids a broken build).

- [ ] **Step 4: Run test + build to verify they pass**

Run: `go test ./internal/contract/... && go build ./cmd/engine`
Expected: PASS, and `cmd/engine` builds cleanly.

- [ ] **Step 5: Commit**

```sh
git add cmd/engine internal/contract
git rm main.go
git commit -m "feat: assemble engine pipeline and replace hello-world main.go"
```

---

## Task 14: End-to-end smoke test

**Files:**
- Test: `internal/e2e/e2e_test.go`

**Interfaces:**
- Consumes: every package + the same assembly as `cmd/engine/main.go`, but with fake models and `AUTO_APPROVE` semantics.
- Produces: a single test that assembles the full `sequentialagent` pipeline with fake models (`brief`→`results`), runs it via `runner.New` with artifact + memory services, and asserts a landing-page artifact exists in the artifact service and `build_output` was written. No real API key.

- [ ] **Step 1: Write the failing test**

```go
// internal/e2e/e2e_test.go
package e2e_test

import (
	"context"
	"testing"

	"github.com/captain-corgi/go-adk-example/internal/brain"
	"github.com/captain-corgi/go-adk-example/internal/build/graph"
	"github.com/captain-corgi/go-adk-example/internal/keys"
	"github.com/captain-corgi/go-adk-example/internal/routing"
	"github.com/captain-corgi/go-adk-example/internal/stages"
	"github.com/captain-corgi/go-adk-example/internal/stages/brief"
	"github.com/captain-corgi/go-adk-example/internal/stages/ideation"
	"github.com/captain-corgi/go-adk-example/internal/stages/research"
	"github.com/captain-corgi/go-adk-example/internal/stages/results"
	"github.com/captain-corgi/go-adk-example/internal/stages/signoff"
	"github.com/captain-corgi/go-adk-example/internal/stages/synthesis"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/v2/artifact"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func TestPipelineShipsLandingPage(t *testing.T) {
	ctx := context.Background()
	const app = "engine_e2e"

	// Fakes: every LLM stage returns valid JSON for its contract; the drafter
	// returns a valid landing page; the checker never exits (max_iter is fine).
	jsonLLM := &canned{Text: `{"ok":true}`}            // for brief/ideation/research/synthesis
	drafterLLM := &canned{Text: "# Hero\nOffer: free\nCTA: Sign up\nA/B: alt"}
	checkerLLM := &canned{Text: `{"critique":"x","fixes":[]}`}
	m := routing.NewModels(checkerLLM, drafterLLM)

	mem := memory.InMemoryService()
	art := artifact.InMemoryService()
	b, err := brain.New(t.TempDir(), mem)
	if err != nil {
		t.Fatal(err)
	}

	briefAg, _ := brief.New(stages.Config{Model: jsonLLM})
	ideationAg, _ := ideation.New(stages.Config{Model: jsonLLM})
	researchAg, _ := research.New(stages.Config{Model: jsonLLM}, b)
	synthesisAg, _ := synthesis.New(stages.Config{Model: jsonLLM})
	signoffAg, _ := signoff.New(true) // AUTO_APPROVE
	buildAg, _ := graph.New(m, 2)
	resultsAg, _ := results.New(mem)

	// jsonLLM must serve the JSON stages AND be the cheap model the drafter
	// uses inside build. To keep them separate, build uses m (drafter/checker),
	// and the upstream stages use jsonLLM directly (already wired above).
	pipeline, err := sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name: "marketing_engine",
			SubAgents: []agent.Agent{
				briefAg, ideationAg, researchAg, synthesisAg, signoffAg, buildAg, resultsAg,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ss := session.InMemoryService()
	created, _ := ss.Create(ctx, &session.CreateRequest{AppName: app, UserID: "u1"})
	sess := created.Session
	r, err := runner.New(runner.Config{AppName: app, Agent: pipeline, SessionService: ss, ArtifactService: art, MemoryService: mem})
	if err != nil {
		t.Fatal(err)
	}

	state := map[string]any{}
	for ev, err := range r.Run(ctx, "u1", sess.ID(),
		genai.NewContentFromText("A no-code analytics tool for indie hackers", genai.RoleUser),
		agent.RunConfig{}) {
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		if ev != nil && ev.Actions != nil {
			for k, v := range ev.Actions.StateDelta {
				state[k] = v
			}
		}
	}

	if _, ok := state[keys.Build]; !ok {
		t.Fatalf("build_output not produced; state=%v", state)
	}
	// The landing-page artifact must exist in the artifact service.
	list, err := art.List(ctx, &artifact.ListRequest{AppName: app, UserID: "u1", SessionID: sess.ID()})
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	found := false
	for _, n := range list.FileNames {
		if n == "landing_page.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("landing_page.md artifact missing; files=%v", list.FileNames)
	}
}

// canned is a local model.LLM returning one fixed text (kept local to avoid an
// import cycle on the test-only stagetest package from an e2e package).
type canned struct{ Text string }

func (c *canned) Name() string { return "canned" }
func (c *canned) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{Content: genai.NewContentFromText(c.Text, "model")}, nil)
	}
}
```

Add imports `iter`, `google.golang.org/adk/v2/model`. Note: the `ListResponse` field for filenames may be `FileNames` — if the actual field differs, read it from the artifact package and adjust (it is the slice of names returned by `List`). Also confirm `ListRequest` fields (`AppName, UserID, SessionID`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/e2e/...`
Expected: FAIL (package/test do not exist, or a wiring detail surfaces).

- [ ] **Step 3: Make it pass**

No new production code is required — this test wires existing components. If it fails, the likely causes are: (a) an `OutputKey` not flowing because a stage's `IncludeContents` defaulted (re-check every stage sets `IncludeContentsNone`); (b) the drafter not seeing `plan_output` because synthesis wrote it but the build sub-agent runs in a fresh turn — ensure `plan_output` is in shared session state (it is, via the sequentialagent's shared session); (c) a wrong field name on `artifact.ListResponse`. Fix the specific cause; do not weaken the assertions.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/e2e/...`
Expected: PASS — full pipeline, fake models, landing-page artifact present, no API key.

- [ ] **Step 5: Commit**

```sh
git add internal/e2e
git commit -m "test: add end-to-end smoke test (fake models, landing-page artifact)"
```

---

## Task 15: Docs, env example, lint, tidy

**Files:**
- Modify (or create): `.env.example`
- Modify: `README.md` (if present) — add a run command.

**Interfaces:** none (housekeeping to reach Definition of Done).

- [ ] **Step 1: Update `.env.example`**

Append the new variables (keep existing ones; never include real keys):

```sh
# --- Marketing Engine (Phase 0) ---
# Model names. Default to OPENAI_MODEL when unset (Phase 0: one model, two aliases).
MODEL_STRONG=
MODEL_CHEAP=
# Bypass the human sign-off gate (CI / non-interactive runs).
AUTO_APPROVE=false
# Max drafter→checker iterations per deliverable before shipping best draft.
MAX_LOOP_ITER=3
# Directory of frozen brand Markdown files (voice.md, offers.md, icp.md, ...).
GBRAIN_DIR=./brand
```

- [ ] **Step 2: Add a run section to README**

Append:

````markdown
## Run the Marketing Engine

```sh
go run ./cmd/engine
```

Set `OPENAI_API_KEY` / `OPENAI_BASE_URL` / `OPENAI_MODEL` (and optionally
`MODEL_STRONG`, `MODEL_CHEAP`, `AUTO_APPROVE=true`, `MAX_LOOP_ITER`, `GBRAIN_DIR`)
in a gitignored `.env`. Submit a one-line idea at the prompt; the engine ships a
landing-page Markdown artifact and prints its location.
````

- [ ] **Step 3: Run the full quality gate**

Run:
```sh
gofmt -w .
goimports -w .
go mod tidy
go build ./...
go vet ./...
go test ./...
golangci-lint run
```
Expected: all clean, all tests green. Address every finding (unused imports, missing `//nolint` justifications are NOT acceptable — fix the code).

- [ ] **Step 4: Commit**

```sh
git add .env.example README.md go.mod go.sum
git commit -m "docs: document Phase 0 env vars and engine run command"
```

---

## Self-Review

**1. Spec coverage** (Phase 0 spec §2 components → task):
- §2.1 config (env + `routing.Models`) → **Task 2** (+ Task 1 for routing).
- §2.2 brain (frozen brand files + `memory.Service` + Load) → **Task 6**.
- §2.3 stages: brief → T4, ideation → T5, research → T7, synthesis → T8, signoff (HITL) → T12, results (memory seam) → T12.
- §2.4 build: deliverables/landingpage → T9; loop → T10; graph → T11.
- §2.5 `cmd/engine/main.go` entry point → **Task 13**.
- §3 data flow → exercised by T14 E2E.
- §4 error handling: non-fatal research/ideation (stages just write output); eval-loop non-convergence → `max_iter_reached` (T10/T11); sign-off rejection path exists (T12 interactive); missing gBrain files non-fatal (T6).
- §5 testing: stage units w/ fake model (T4–T8), deliverable unit (T9), eval loop pass/max_iter (T10), contract test (T13), E2E smoke no-key (T14). ✅ All five present.
- §6 config additions → Task 15 (.env.example). ✅
- §7 DoD: build/test/vet/lint clean → T15; real-key AUTO_APPROVE run produces artifact → T13 builds the binary, manual run documented; every contract §3 has ≥1 impl + test → covered; build graph + loop structured for a 2nd deliverable as a new edge/file → T9 (new `Deliverable`), T10 (generic `LoopSpec`), T11 (`AddFanOut`/`AddFanIn`). ✅

**2. Placeholder scan:** None of "TBD/TODO/implement later/add error handling" remain in task bodies. Two intentional TODO-style lines exist in *source comments* documenting upstream ADK behavior (`// TODO` is not in our code). Every code step shows real code.

**3. Type consistency:** Verified across tasks:
- `routing.Models.Strong()/Cheap()/For(Role)` used identically in T1, T10, T11, T13, T14.
- `stages.Config{Model, Brand}` defined T3, used T4/T5/T7/T8/T13/T14.
- `stagetest.RunAgent(...)` signature stable (T3) and called identically T4/T5/T7/T8/T10/T12/T13.
- State keys all come from `internal/keys` (T1) — no string-literal drift.
- `loop.LoopSpec` (T10) is satisfied by `*landingpage.LandingPage` accessors (T9): `DrafterInstruction/CheckerInstruction/DraftKey/CritiqueKey`. ✅
- `deliverables.BuildInput{Brief,Plan,Research,Brand json.RawMessage; Draft string}` defined T9, consumed T11. ✅
- `graph.New(models, maxIter)` defined T11, called T13/T14 with `routing.Models`. ✅

**Residual risks (documented, not blockers):**
- The openaimodel v2.1.0 multi-turn bug could still 400 on a real-key run despite `IncludeContentsNone`. All tests use fakes and are unaffected. If a real run 400s on stage 2+, this is that known bug — report it; the state-passing design is the maximum mitigation available in v2.1.0. Phase 5 re-validates.
- Exact field names on `artifact.ListResponse` (`FileNames`) and `ListRequest` are stated from the survey; T14 instructs verifying against the package and adjusting if different.
- The interactive sign-off HITL path is unit-tested only via AUTO_APPROVE; the resume path is verified manually (the runner auto-resumes from the next user message).
