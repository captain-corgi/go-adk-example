# The Marketing Engine — Overview & Shared Contract

**Status:** Living document. Phase 0 is authoritative; phases 1–5 are **provisional** (revisited when each is reached).
**Source vision:** `docs/AI-marketing-engine.png` ("THE MARKETING ENGINE" by @shannholmberg).
**Intent:** Learning exercise on `google.golang.org/adk/v2` (adk-go) that must **genuinely work end-to-end** — one raw idea in, a shipped campaign out.

---

## 1. What we're building

A multi-agent marketing campaign factory. The diagram is the destination; we build it as a sequence of **working** sub-projects (each its own spec → plan → build cycle), because the pipeline must exist before later verticals can plug into it. The diagram itself is built on *"one graph, swappable pieces at each step"* — so swappability is the design, not rework.

### Phased roadmap (each phase ships a working increment)

| Phase | Ends with a working… | Boxes / verticals landed |
|---|---|---|
| **0 — Spine** | End-to-end run: idea → brief → ideation → research → synthesis → sign-off → **build** → **1 deliverable** (landing page) via an eval loop, with a file-based gBrain feeding context. | Whole pipeline once; model routing (1 model ok); **landing page**; LOOP primitive proven. |
| **1 — Swappable build** | Deliverable-GRAPH and eval-LOOP made first-class & pluggable; add **social, email, paid ads**. | +3 verticals (4/8). |
| **2 — Remaining text verticals** | **Blog/SEO, guides/magnets, PR/earned** as nodes. | All text verticals (7/8). |
| **3 — Live research + memory loop** | Real external research (web); **Results → gBrain** writes learnings back. | External research + feedback loop, real. |
| **4 — Video** | **Video** vertical: scripts/storyboards/shot-lists always; renders *if* a video-model endpoint is wired. | 8/8. |
| **5 — Real routing + polish** | True multi-model routing (strong→thinking, cheap→grunt), hardened sign-off UX, telemetry. | Model routing + delegation, for real. |

---

## 2. ADK v2.1.0 primitive map

Grounded in the installed module (`go env GOMODCACHE` → `google.golang.org/adk/v2@v2.1.0`).

| Diagram concept | ADK primitive |
|---|---|
| Pipeline (raw idea → … → results) | `agent/workflowagents/sequentialagent` |
| **LOOP** (draft → check → fix) | `agent/workflowagents/loopagent` (+ `tool/exitlooptool`) |
| Parallel fan-out of deliverables | `agent/workflowagents/parallelagent` and/or `workflow/parallel_worker` |
| **GRAPH** (nodes + routes) | `workflow` (`graph`, `branch`, `agent_node`, `function_node`, `tool_node`, `join_node`, `dynamic_node`) |
| Human sign-off (HITL) | `workflow` HITL (`request_input`/`resume`), `tool/toolconfirmation`, `agent.BeforeAgentCallback` |
| **gBrain** (frozen context) | `artifact` service + `Instruction` `{artifact.key}` templating; `tool/loadartifactstool` |
| **gBrain** (accumulated memory) | `memory.Service` (`AddSessionToMemory` / `SearchMemory`) + `tool/loadmemorytool` |
| Model routing | per-agent `llmagent.Config.Model`; `llmagent.BeforeModelCallback` for intercept/routing |
| Stage I/O | session **state** keys + `llmagent` structured output (`SaveOutput`) |
| External research | `tool/geminitool/google_search` (Gemini) or a `tool/functiontool` wrapping a search API |
| Deliverable storage | `artifact` service (save deliverables as artifacts) |
| Sub-agent delegation | `llmagent.Config.SubAgents`, `tool/agenttool` |

---

## 3. Shared plug-in contract (the "swappable pieces")

This is the interface every phase implements against. Phase 0 freezes it; later phases add implementations. Two small, **additive** shape extensions are introduced later and noted inline: an optional `Loopable` interface (Phase 1, §3.4) and a sibling `Renderer` registry (Phase 4, §3.7) — neither breaks the base `Deliverable`.

### 3.1 Package layout

```text
cmd/engine/main.go              # build agents, run launcher
internal/
  config/                       # env, model construction (Models registry)
  brain/                        # gBrain: load brand artifacts + memory service wiring
  stages/                       # pipeline stage agents
    brief/ ideation/ research/ synthesis/ signoff/
  build/
    loop/                       # eval loop (draft→check→fix)
    graph/                      # deliverable graph (fan-out → join)
    deliverables/               # one package per vertical: landingpage/, social/, email/, ...
  routing/                      # role→model resolution
brand/                          # gBrain frozen source: voice.md, offers.md, icp.md, sops/, wins/
docs/superpowers/specs/         # these specs
```

### 3.2 Stage I/O contract (most important)

Every pipeline stage is an `llmagent` (or `workflow` node) that:
- **reads** prior outputs from session state,
- **writes** a single typed JSON blob to a namespaced **state key**.

| Stage | Reads | Writes (state key) |
|---|---|---|
| Brief | user input (`{user_query}`) | `brief.output` |
| Ideation | `brief.output` | `ideation.output` |
| Research | `brief.output`, gBrain | `research.output` |
| Synthesis | `brief` + `ideation` + `research` | `plan.output` |
| Sign-off | `plan.output` | `signoff.output` (approved/edited plan) |
| Build | `signoff.output` | `build.output` + deliverable artifacts |
| Results | `build.output` | memory entry + `results.output` |

State keys are the **only** coupling between stages. Any stage can be swapped or reordered by honoring its input key and producing its output key.

### 3.3 Deliverable interface

Every vertical implements one interface so the build graph can route to any of them identically:

```go
// internal/build/deliverables
type Deliverable interface {
    Name() string // "landing_page", "social", "email", ...
    Build(ctx context.Context, in BuildInput) (*Artifact, error)
}

type BuildInput struct {
    Brief    json.RawMessage
    Plan     json.RawMessage
    Research json.RawMessage
    Brand    json.RawMessage // gBrain frozen context for this vertical
}
```

Phase 0 ships one implementation (`landingpage`); phases 1–2 add the rest. Each is unit-testable in isolation.

### 3.4 Eval loop contract

`loopagent` wrapping **drafter → checker → (loop)**, where the checker calls `exitlooptool` when the deliverable passes its quality protocol, else returns fixes that flow back to the drafter. `MaxIterations` is bounded (default from `MAX_LOOP_ITER`). Used per-deliverable.

> **Phase 1 extension:** an optional `Loopable` interface (`Loop() LoopSpec`) lets each vertical supply its own drafter/checker instructions, output schema, and iteration cap. `Deliverable` is unchanged; `Loopable` opts a vertical into the loop.

### 3.5 Build graph contract

A `workflow` graph: `signoff.output` → fan-out to N deliverable nodes, each wrapped in the eval loop → `join` (`workflow.JoinNode`) → `build.output` + artifacts. N=1 in Phase 0; grows each phase.

> **Phase 1 refinement (authoritative):** deliverable fan-out uses **one explicit graph edge per deliverable into a single `JoinNode`**, *not* `ParallelWorker`. `workflow.ParallelWorker` is fail-fast and wraps one node — incompatible with partial-success across independent loop-wrapped verticals. `ParallelWorker` is still the right primitive *inside* a long-form vertical (Phase 2) for per-section fan-out with branch isolation. The registry seam means swapping to `ParallelWorker` later (dynamic N) is local to `internal/build/graph`.

### 3.6 gBrain contract

Two layers:
- **Frozen context** (voice, brand rules, offers, ICP, SOPs): Markdown files under `brand/`, loaded at startup into the **artifact** service; injected into stage instructions via `{artifact.voice}` etc., and retrievable on demand.
- **Accumulated memory** (past wins, what worked/died): the `memory.Service` (in-memory for dev; swappable to Vertex). Queried at research time; written at results time (`AddSessionToMemory`) — the diagram's feedback loop.

### 3.7 Model routing contract

A `routing.Models` registry resolves a **role** → `model.LLM`:

| Role | Used by | Phase 0 model |
|---|---|---|
| `strong` (thinking/judgment) | synthesis, sign-off eval, checker | `OPENAI_MODEL` |
| `cheap` (grunt work) | drafter, brief, ideation | `OPENAI_MODEL` |
| `video` (renders) | video vertical | n/a until Phase 4 — routes via a sibling `routing.Renderers` registry (video-gen APIs aren't chat `model.LLM`s); see Phase 4 |

Env: `MODEL_STRONG`, `MODEL_CHEAP` (both default to `OPENAI_MODEL`). Phase 0 routes everything to one model; the **seam** (role→model lookup) exists from day one so Phase 5 just changes the mapping.

### 3.8 Sign-off contract

A HITL gate between synthesis and build. Pauses, presents `plan.output`, accepts approval or edits, resumes. Bypassed by `AUTO_APPROVE=true` (still logged) for non-interactive/CI runs.

---

## 4. Environment

Existing: `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_MODEL`.
Added by Phase 0: `MODEL_STRONG`, `MODEL_CHEAP`, `AUTO_APPROVE`, `MAX_LOOP_ITER`, `GBRAIN_DIR`.

**Never hardcode secrets.** Keys stay in gitignored `.env`.

---

## 5. How the specs relate

- `00-marketing-engine-overview.md` (this file) — contract; read first.
- `2026-08-02-phase0-spine-design.md` — authoritative foundation.
- `2026-08-02-phase{1..5}-*-design.md` — provisional; each implemented only when its phase begins.
