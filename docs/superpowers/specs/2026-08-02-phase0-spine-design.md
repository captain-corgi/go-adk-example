# Phase 0 — Spine Design (authoritative)

**Goal:** a genuinely working end-to-end run — **one raw idea in → one landing-page deliverable out** — that exercises every architectural concept in the engine exactly once, on top of the current hello-world scaffold. This phase freezes the [shared contract](./00-marketing-engine-overview.md).

**Out of scope (later phases):** extra verticals, live web research, results→memory writeback beyond the seam, video, real multi-model routing.

---

## 1. Architecture

```text
                 ┌──────────────── sequentialagent (the pipeline) ────────────────┐
user idea ──►   brief ─► ideation ─► research ─► synthesis ─► signoff ─► build ─► results
                   │         │           │            │         │         │         │
                   └─state──►└──state───►└──state────►└─state──►└─state──►└─state──►┘
                                                            ▲                    │
                                                  gBrain (artifacts + memory) ◄──┘ feedback seam
```

- The pipeline is one `sequentialagent` whose sub-agents are the seven stage agents.
- `build` is itself a `workflow` graph containing one deliverable node wrapped in a `loopagent` (the eval loop).
- All inter-stage data flows through session **state keys** (contract §3.2).

## 2. Components

### 2.1 `internal/config` — model construction
- Reads `.env` (existing `godotenv` pattern).
- Builds the `openaimodel` (existing code) and exposes a `routing.Models` registry mapping role→model. Phase 0: `strong` and `cheap` both resolve to `OPENAI_MODEL`.

### 2.2 `internal/brain` — gBrain
- **Frozen context:** loads `brand/*.md` into the artifact service at startup (keys: `voice`, `brand_rules`, `offers`, `icp`, `sops`, `wins`).
- **Memory:** constructs an `inmemory` `memory.Service` (dev). Provides `Load(ctx, query)` used by the research stage.
- Ship a tiny seed `brand/` (a few short Markdown files) so the engine runs out of the box.

### 2.3 Stage agents (`internal/stages/*`)
Each is an `llmagent` with: a role-appropriate `Model`, an `Instruction` that reads its input state key(s) via templating and writes its output key. Instructions emphasize the output schema.

- **brief** — turns a rampled/voiced raw idea into a clean structured brief (`brief.output`).
- **ideation** — generates ~6 angles, scores against gBrain voice/ICP, keeps top 2–3 (`ideation.output`).
- **research** — **internal:** queries gBrain memory + artifacts for relevant past wins/rules. **external:** reads a local market-notes artifact (stub of live search; Phase 3 makes it real). Merges into `research.output`.
- **synthesis** — merges brief + angles + research into a concrete campaign plan: target hook, offer, message hierarchy, CTA, deliverable spec (`plan.output`).
- **signoff** — HITL gate: presents `plan.output`, waits for approval/edits via workflow `request_input`/`resume`. Writes `signoff.output`. `AUTO_APPROVE=true` short-circuits with a logged decision.
- **build** — see 2.4.
- **results** — saves deliverable(s) as artifacts; calls the **seam** `memory.AddSessionToMemory(...)` (logged; Phase 3 enriches).

### 2.4 `internal/build` — loop + graph
- **`deliverables/landingpage`** — implements `Deliverable`: emits hero + offer copy + 3 CTAs + an A/B variant, as a Markdown artifact.
- **`loop`** — `loopagent` over `[drafter, checker]`. Drafter produces the landing page; checker evaluates it against a small quality protocol (on-brand voice, has hero+offer+CTA, ≤N words) and calls `exitlooptool` on pass, else returns critique that the drafter consumes next iteration. `MAX_LOOP_ITER` bounds it (default 3).
- **`graph`** — `workflow` graph: `plan` → `landing_page` node (wrapped in the eval loop) → `join` → `build.output` + artifact. N=1; structure already supports fan-out in Phase 1.

### 2.5 `cmd/engine/main.go` — entry point
Replaces the hello-world `main.go`: builds models, gBrain, stages, build graph, assembles the `sequentialagent`, and runs it via the existing `launcher` (`full.NewLauncher`). The launcher already provides the interactive runner/session services.

## 3. Data flow (happy path)

1. User submits a raw idea (`{user_query}`).
2. **brief** → `brief.output` (structured brief).
3. **ideation** reads `brief.output` + `{artifact.voice}`/`{artifact.icp}` → `ideation.output` (top angles).
4. **research** reads `brief.output`, calls `brain.Load` (memory + artifacts), reads market-notes artifact → `research.output`.
5. **synthesis** reads brief+ideation+research → `plan.output`.
6. **signoff** presents plan → user approves → `signoff.output`.
7. **build** graph: landing-page eval loop runs draft→check→fix → landing-page artifact + `build.output`.
8. **results** persists artifact, writes memory seam, prints location of the shipped landing page.

## 4. Error handling

- **Model/API errors:** surface non-fatal per-stage errors as stage output with `error` field; a stage that errors writes `*.output = {error: ...}` and the pipeline continues where safe (research/ideation are non-fatal; synthesis/build/signoff errors abort with a clear message). Use `OnModelErrorCallback` for one retry then fail.
- **Eval loop non-convergence:** if `MAX_LOOP_ITER` reached without pass, ship the best draft with a `quality: "max_iter_reached"` flag rather than failing silent.
- **Sign-off rejection:** if the user rejects, end the run cleanly with the rejected plan logged to memory (a "what died" signal).
- **Missing gBrain files:** log a warning and continue with empty context (engine must still run).

## 5. Testing

- **Stage units:** each stage tested with a fake `model.LLM` returning canned content; assert it reads the right input key and writes a valid `*.output`.
- **Deliverable unit:** `landingpage.Build()` with fixed `BuildInput` → assert artifact has hero+offer+CTA sections.
- **Eval loop:** test (a) checker passes on iter 2 → exits, (b) never passes → returns best draft with `max_iter_reached`.
- **Contract test:** a single test asserting every stage publishes/accepts its contracted state keys (guards swappability).
- **E2E smoke:** `AUTO_APPROVE=true` + fake model → run full pipeline → assert a landing-page artifact exists. No real API key required for tests.

## 6. Config additions

`MODEL_STRONG`, `MODEL_CHEAP` (default `OPENAI_MODEL`), `AUTO_APPROVE=false`, `MAX_LOOP_ITER=3`, `GBRAIN_DIR=./brand`. Document in `.env.example`.

## 7. Definition of done (Phase 0)

- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all clean.
- Running the engine with a real key + `AUTO_APPROVE=true` produces a landing-page Markdown artifact from a one-line idea, end to end, with gBrain context visibly reflected in the copy.
- Every contract in overview §3 has at least one implementation and one test.
- The build graph and eval loop are structured so adding a second deliverable (Phase 1) is a new file, not a refactor.
