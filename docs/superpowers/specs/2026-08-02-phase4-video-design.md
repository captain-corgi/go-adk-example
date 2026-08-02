# Phase 4 — Video Vertical Design (provisional)

**Goal:** close the 8/8 vertical map. Ship **always-text** video artifacts (scripts, storyboards, shot lists) through the standard build path, and produce **actual video renders only if** a video/image-generation endpoint is configured. Text ships unconditionally; renders are a decoupled, best-effort side-effect.

**Open dependency (flagged):** access to a video-generation model/endpoint is unknown at design time. The default design ships text artifacts with **zero** video access; the render tier is a seam that activates purely from env config. Nothing below assumes any specific vendor.

---

## 1. Architecture

Two tiers, decoupled by an interface so the text tier never depends on render-tier availability.

```
signoff.output ──► [build graph fan-out (Phase 1/2)] ──┐
                                                       ├──► video (text) node ──► render node (optional) ──► join ──► build.output + artifacts
                                                       │     tier 1: always            tier 2: gated          │
                                                       └──► (other verticals) ─────────────────────────────┘
```

- **Tier 1 — Text (always):** `internal/build/deliverables/video` implements `Deliverable` (contract §3.3) like any vertical. Its node runs inside the standard `loopagent` eval loop (drafter→checker). Output is Markdown: a video script (UGC ad / explainer / VSL variant selected from `plan.output`), a storyboard, and a shot list.
- **Tier 2 — Render (optional):** a separate `function_node` placed *after* the video text node and *outside* the eval loop. The loop judges text quality; rendering is a side-effect on the settled script. The node calls a `Renderer`; with no endpoint configured it returns a skipped status and the run proceeds.

The render node is a `workflow` `function_node` (not an `agent_node`) — it performs a deterministic side-effect, needs no model, and must never re-enter the loop.

## 2. Components

### 2.1 `internal/build/deliverables/video` — the `Deliverable`
- `Name()` → `"video"`. `Build(ctx, BuildInput)` emits an `*Artifact` whose Markdown contains: chosen format, script (with VO/on-screen/CTA columns), storyboard (scene → visual → audio), shot list (shot #, type, duration, b-roll notes).
- Drafter instruction selects format from `plan.output`; checker protocol asserts all three sections present, on-brand voice (via gBrain `{artifact.voice}`), CTA present, ≤ target word budget.

### 2.2 `Renderer` interface + implementations (`internal/build/render`)
```go
type Renderer interface {
    Render(ctx context.Context, req RenderRequest) (*RenderResult, error)
}
type RenderRequest struct { Script, Storyboard string; Plan json.RawMessage }
type RenderResult struct {
    Status   string            // "rendered" | "skipped" | "failed"
    Artifact *genai.Part       // inline-data bytes when rendered; nil otherwise
    Note     string            // human-readable reason (e.g. "no endpoint")
    Cost     *CostRecord       // optional, for quota guard
}
```
- **`noopRenderer`** (default): returns `Status:"skipped"`, `Note:"no VIDEO_ENDPOINT configured"`. Constructed when `VIDEO_ENABLED` falsy or endpoint unset.
- **`httpAPIRenderer`**: bound to `VIDEO_ENDPOINT`/`VIDEO_MODEL`/`VIDEO_API_KEY`. Video-gen APIs are **async** (submit → poll → fetch), which does not conform to `model.LLM.GenerateContent`'s chat-completion iter shape — so the renderer is a plain HTTP client, not an `model.LLM`. Submit+poll runs under one `ctx` deadline; on terminal success it fetches bytes and wraps them in a `*genai.Part` (inline data + MIME) for the artifact service.

### 2.3 Routing — the `video` role (contract §3.7)
The `video` role is wired into the routing layer as a named constant. **Deliberate deviation:** because generation endpoints are not chat LLMs, the role resolves through a new sibling registry `routing.Renderers` (role→`Renderer`), **not** `routing.Models` (role→`model.LLM`, whose `GenerateContent` signature is chat-shaped). This honors §3.7's *seam* while respecting real type shapes. `routing.Renderers["video"]` returns `noopRenderer` unless `VIDEO_ENABLED=true` and `VIDEO_ENDPOINT` set.

### 2.4 Config gating
`internal/config` constructs the `Renderer` from env at startup and registers it. The text tier is built unconditionally — it has no knowledge of whether a renderer is configured.

## 3. Data flow (happy path)
1. `signoff.output` reaches the build graph; fan-out schedules the video node alongside other verticals.
2. **Video text node** runs the eval loop → settled Markdown script/storyboard/shot-list → saved as artifact `video_script.md` via `artifact.Service.Save`; written to its slot in `build.output`.
3. **Render node** (function_node) reads the settled text, calls `routing.Renderers["video"].Render(ctx, ...)`.
4. On `rendered`: bytes saved as artifact `video_render.<ext>`; `build.output` records `video.render.status="rendered"` + cost. On `skipped`/`failed`: status recorded, text artifact already shipped.
5. `join` merges all verticals → `build.output` → `results`.

## 4. Error handling
- **Render failure/timeout never blocks text.** The render node recovers from any renderer error and records `status:"failed"` + `note`; the pipeline continues. The text artifact is already persisted at step 2, before render.
- **Quota/cost guard (preflight + reconcile):** before submitting the render, reserve/estimate the cost against `VIDEO_MAX_COST_PER_RUN` (and a daily counter) and skip early (`status:"skipped"`) if the budget would be exceeded — the provider may otherwise accept and charge the request. After the render, reconcile against `RenderResult.Cost` and downgrade to `skipped` with a logged note on overrun.
- **Async polling:** bounded retries + `ctx` deadline (`VIDEO_RENDER_TIMEOUT`); terminal non-success states surface as `failed`, not hangs.
- **Missing `VIDEO_ENDPOINT` with `VIDEO_ENABLED=true`:** config-load warning, falls back to `noopRenderer` (engine still runs).

## 5. Testing
- **Text-tier unit:** `video.Build()` with fixed `BuildInput` → assert script+storyboard+shot-list sections and on-brand voice — identical pattern to other verticals.
- **Eval loop:** reuse contract loop tests; checker passes on iter 2 / `max_iter_reached` ships best draft.
- **`noopRenderer`:** returns `skipped` + nil artifact.
- **`httpAPIRenderer`:** against `httptest.Server` simulating submit→poll(pending)→poll(done)→fetch bytes; assert `RenderResult.Artifact` carries the bytes + MIME. Plus timeout and failure-variant cases.
- **Gating test:** missing `VIDEO_ENDPOINT` → text ships, render `skipped` + logged note; `build.output` still complete. This is the must-pass contract test.

## 6. Config additions
`VIDEO_ENABLED=false`, `VIDEO_ENDPOINT`, `VIDEO_MODEL`, `VIDEO_API_KEY`, `VIDEO_RENDER_TIMEOUT` (e.g. 180s), `VIDEO_MAX_COST_PER_RUN`. Documented in `.env.example`; keys never hardcoded.

## 7. Definition of done

**Level 1 — Text tier done (always reachable, the bar for merge):**
- `go build/test/vet`, `golangci-lint run` clean.
- With no video env set, a run produces `video_script.md` (script+storyboard+shot-list) from `signoff.output`, end to end, through the standard eval loop, alongside the Phase 1/2 verticals. Render status is `skipped` + logged.
- `Renderer` interface, `noopRenderer`, and `routing.Renderers` seam exist and are tested.

**Level 2 — Renders done (only if an endpoint is available):**
- With `VIDEO_ENABLED=true` + `VIDEO_ENDPOINT` + key set, a run additionally produces a `video_render.<ext>` artifact; `build.output` reports `rendered` + cost.
- `httpAPIRenderer` passes the `httptest` suite; a real-vendor smoke run is recorded (vendor-specific; not a merge gate).

A run that ships text with `render:skipped` satisfies Level 1 and unblocks Phase 5.
