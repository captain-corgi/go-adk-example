# Phase 2 — Remaining Text Verticals Design (provisional)

**Goal:** add the last 3 text verticals — **blog/SEO** (pillar articles, how-to guides, comparison pages), **guides/magnets** (ebooks, checklists, templates, whitepapers), **PR/earned** (press releases, media pitches, founder bylines). After Phase 2 all 7 text verticals are in place (8/8 arrives with video in Phase 4). Assumes Phase 1's swappable build graph and per-vertical eval loops are deployed.

**Out of scope:** live keyword/SEO research data (Phase 3), video (Phase 4), real multi-model routing (Phase 5).

---

## 1. Architecture

These deliverables are longer-form than Phase 1's. A pillar article or ebook can't be produced in a single model call; its flow is **outline → draft sections in parallel → assemble**. The key design move: a `Deliverable` may internally be a nested `workflow.Workflow`, exposed to the build graph as a single `workflow.WorkflowNode`, while still satisfying contract §3.3.

Two coexisting patterns (both implement `Deliverable`):

- **Single-agent vertical** (Phase 1 style, for short PR assets): build-graph node = a `loopagent` over `[drafter llmagent, checker llmagent]`.
- **Long-form vertical** (new): build-graph node = a `loopagent` over `[drafter=WorkflowNode, checker llmagent]`. The `WorkflowNode` (`workflow.NewWorkflowNode`) wraps an internal graph; `WorkflowNode.Run` forwards only the single terminal node's output (ADK enforces ≤1 terminal output via `ErrMultipleOutputs`), so the nested graph behaves like an agent to its parent.

Internal long-form graph (blog pillar / ebook):

```
Start ─► outline(AgentNode) ─► sections(ParallelWorker) ─► assemble(FunctionNode)
                                   ▲
              drafter llmagent wrapped per item, branch-isolated (ParallelWorker derives name@i sub-branches)
```

- `outline` — `llmagent` emits a JSON list of sections (title + summary + target keyword).
- `sections` — `workflow.NewParallelWorker(name, sectionDrafterNode, LONGFORM_SECTION_CONCURRENCY, cfg)` runs the section drafter once per slice element; per-item branch isolation keeps each drafter's history scoped independently.
- `assemble` — `workflow.NewFunctionNode` consumes its predecessor's slice via `node_input` and concatenates sections in outline order into one Markdown artifact.

The `Deliverable` contract (`Name`, `Build(ctx, in) (*Artifact, error)`) is **unchanged**. For unit tests / direct calls, `Build` constructs the same internal `Workflow` and drives it through a runner injected at the vertical's constructor (DI, not a contract change). The build-graph wiring asks each vertical for its drafter node and wraps it in the shared eval-loop `loopagent`.

## 2. Components

`internal/build/deliverables/{blogseo,guides,pr}`, each implementing `Deliverable`:

- **`blogseo`** — three sub-types in one package: `Pillar`, `HowTo`, `Comparison`. `Pillar`/`HowTo` use the nested long-form graph above (bounded by `LONGFORM_MAX_SECTIONS`); `Comparison` is shorter and uses the single-agent path with table-emphasis instructions. Quality protocol (checker): primary keyword appears in H1 + intro + ≥1 H2; H1/H2 hierarchy present; meta description ≤155 chars; word count in range. **Provisional/dependency:** keyword *research* (search volume, difficulty, related terms) remains a stub — the brief supplies the target keyword and the checker only validates presence/structure. Real keyword data arrives with Phase 3's live search (`tool/geminitool/google_search` or a `functiontool`).
- **`guides`** — `Ebook` uses the long-form graph (chapters); `Checklist`, `Template`, `Whitepaper` default to the single-agent path (short enough); `Ebook.assemble` additionally produces a table of contents + cover copy. Checker: section count, CTA presence, gated-asset CTA (driven by the `Deliverable` spec).
- **`pr`** — `PressRelease`, `MediaPitch`, `FounderByline`. All single-agent (inverted-pyramid / one-pitch / first-person). No nested graph. Three drafter nodes in-package fan out via the PR entry node's `EdgeBuilder.AddFanOut` and join at a `NewJoinNode`, so each build emits a PR "package" artifact. Checker: AP-style lede, quote present, contact block, boilerplate.

Each vertical registers its quality protocol with the shared `loop` (contract §3.4); `MAX_LOOP_ITER` still bounds refinement.

## 3. Data flow

These verticals slot into the Phase 1 build fan-out graph exactly like the existing nodes: `signoff.output` → fan-out (explicit per-vertical edges into a `JoinNode`, per Phase 1) across all registered verticals → each wrapped in its eval-loop `loopagent` → `build.output` + artifacts. The build graph need not know which verticals are nested graphs vs single-agent — both are just a `Node` that yields one terminal output. Vertical activation follows `plan.output`'s deliverable spec (Phase 1 routing).

## 4. Error handling

- **Partial success for long-form sections:** raw `ParallelWorker` is fail-fast, which is wrong here. Each section-drafter wrapper catches model/API errors and returns `{section_index, body?, ok, err}` instead of propagating. `assemble` substitutes a short `"[Section unavailable: <title>]"` placeholder for failed sections, sets `quality.degraded=true` with `degraded_sections=[...]`, and the article ships degraded — never aborts on one section. Failures are logged to memory via the results sink.
- **Outline failure:** if `outline` itself fails, the deliverable falls back to a single-agent one-shot draft (graceful degradation to Phase 1 behavior), flagged `degraded=true`.
- **Eval-loop non-convergence and API errors:** inherit Phase 0 semantics (`max_iter_reached` ships best draft; `OnModelErrorCallback` retries once then surfaces).
- **SEO keyword stub missing:** if the brief carries no target keyword, the checker skips the keyword check with `quality.keyword_check="skipped_no_data"` (logged) — never fails the loop.

## 5. Testing

- **Per-vertical unit:** `Build()` with a fixed `BuildInput` + fake `model.LLM`; assert artifact shape (pillar has N sections + H1 + keyword; press release has lede+quote+boilerplate).
- **Nested sub-graph end-to-end:** fake model returns a 3-section outline + 3 section bodies → assert `assemble` ordering, terminal-output forwarding, and that the `WorkflowNode` exposes exactly one output.
- **Partial-failure test:** one section drafter errors → assert a degraded artifact with placeholder + `degraded=true`, and the pipeline does not abort.
- **SEO checker:** keyword present/absent and missing-H2 cases both exit/pass the loop correctly.
- **Contract test:** extend the §3.3 contract test to register all 7 text verticals and assert each emits a `build.output` artifact.

## 6. Config additions

`LONGFORM_MAX_SECTIONS=5`, `LONGFORM_SECTION_CONCURRENCY=0` (0 = unbounded), `SEO_KEYWORD_STUB=true`, `PR_VERTICALS=press_release,media_pitch,founder_byline` (comma-list enable set). Documented in `.env.example`.

## 7. Definition of done (Phase 2)

- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all clean.
- With a real key + `AUTO_APPROVE=true`, a one-line idea produces a blog pillar Markdown artifact (multi-section, assembled) end to end, and demonstrates degraded-section handling.
- All 7 text verticals are registered into the build graph; adding the 8th (video, Phase 4) is a new file, not a refactor.
- The `WorkflowNode`-based nested sub-graph pattern has one implementation and one end-to-end test; SEO keyword research is explicitly marked **provisional/dependency**, pending Phase 3.
