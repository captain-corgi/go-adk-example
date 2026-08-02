# Phase 3 — Live Research + Memory Loop Design (provisional)

**Goal:** make two Phase-0 seams real — (a) the research stage's **external** web-search stub, and (b) the **Results → gBrain** feedback loop (campaigns distill what worked / what died / winning hooks into memory that the next campaign's research stage retrieves). Honors the [shared contract](./00-marketing-engine-overview.md) §3 and the [Phase 0](./2026-08-02-phase0-spine-design.md) section layout. No new state-key shapes — only new implementations behind existing seams.

**Open dependency (flagged):** it is unknown which web-search provider/key is available (Tavily / Serper / Bing / none). **The default design must run with no key** — it degrades to the Phase 0 local market-notes stub. `SEARCH_PROVIDER` selects the live backend once a key exists.

---

## 1. Architecture

```
research stage ── internal: gBrain memory (SearchMemory) + artifacts ──┐
                 external: ResearchProvider.Search(query) ─────────────┤── merge ── research.output
                                                                        │
        ... pipeline ...                                               │
                 └──► results stage ── extract learnings ── AppendEvent ──► AddSessionToMemory  (feedback seam, closed)
```

**(a) ResearchProvider** — a swappable interface (§2.1). Default impl is a `functiontool`-style wrapper over a search REST API (Tavily/Serper/Bing, selected by `SEARCH_PROVIDER`, keyed by `SEARCH_API_KEY`). Alternative: ADK's `tool/geminitool/google_search.GoogleSearch{}` attached directly to the Gemini-backed research `llmagent.Config.Tools` (no key handling on our side — the model invokes its built-in Google Search). A `stubProvider` returns the Phase 0 market-notes artifact text. The factory `NewFromEnv()` picks the implementation; with no key/env it returns the stub and logs a warning. The engine always runs.

**(b) Memory loop** — the results stage gains a **learnings-extraction** sub-step. It produces a structured textual summary (what worked, what died, winning hook, vertical, audience, one-line verdict) as the sub-step's `llmagent` output, which the runner already persists as a session `Event` with `LLMResponse.Content`. The stage then calls `memory.Service.AddSessionToMemory(ctx, session)`. Because `inmemory.go` indexes `event.LLMResponse.Content.Parts[].Text` words, the learnings text doubles as the retrieval payload — no parallel store needed. Retrieval at research time uses `memory.Service.SearchMemory` (via `brain.Load`) or the model-facing `tool/loadmemorytool`.

## 2. Components

### 2.1 `internal/research/provider.go`
```go
type ResearchProvider interface {
    Search(ctx context.Context, query string) (*SearchResult, error)
}
type SearchResult struct{ Snippets []Snippet; Raw []byte; Source string }
```
Implementations: `httpProvider` (one `RESTClient` per `SEARCH_PROVIDER`, with timeout + retry), `geminiProvider` (thin adapter wrapping `geminitool.GoogleSearch` for Gemini setups), `stubProvider` (reads `market-notes` artifact via the artifact service). `NewFromEnv(cfg, artifacts) ResearchProvider` resolves the choice; absent key → stub + `log.Warn`. Each `httpProvider` normalizes its vendor's JSON to `SearchResult`.

### 2.2 `internal/stages/research` (enhanced)
Phase 0 already reads `brief.output` + gBrain; Phase 3 adds: build a query from the brief's hook/ICP, call `ResearchProvider.Search`, merge snippets with gBrain memory + artifacts into `research.output` (adds a `web: [...]` array alongside the existing `internal: {...}`). The provider is injected at construction — the stage depends on the interface, not a vendor.

### 2.3 `internal/stages/results` — learnings extractor
A sub-step (new `llmagent`, role `strong`) reads `build.output` + `signoff.output` + `plan.output`, emits `learnings.output`: a compact text blob with stable labels (`WORKED:`, `DIED:`, `HOOK:`, `VERTICAL:`, `AUDIENCE:`, `VERDICT:`). The results stage then: (1) appends that content as an event via `session.Service.AppendEvent(ctx, session, event)` where `event.LLMResponse.Content = &genai.Content{Parts: []*genai.Part{{Text: learningsText}}}` (built with `session.NewEvent`), (2) calls `memoryService.AddSessionToMemory(ctx, session)`, (3) writes `results.output`. Writes `results.output` even if memory write fails.

### 2.4 Persistence (provisional)
In-memory `memory.Service` is lost on restart, breaking the loop across runs. Propose `MEMORY_BACKEND` (`inmemory`|`vertex`|`file`): `vertex` uses `vertexai.NewService`; `file` is a thin local impl we ship — JSON-lines of `{AppName,UserID,Event}` at `MEMORY_FILE_PATH`, replayed into an `InMemoryService` at startup and appended on each `AddSessionToMemory`. This is provisional — flagged for revisit; default stays `inmemory`.

## 3. Data flow

**Research:** `brief.output` → query → `brain.Load` (memory `SearchMemory` + artifacts) **+** `ResearchProvider.Search` → merged `research.output` (internal + web). **Results:** `build.output` → learnings sub-step → `learnings.output` → `AppendEvent` → `AddSessionToMemory` → `results.output`. Next run's research `SearchMemory` now hits this campaign's learnings — the diagram's feedback loop is live.

## 4. Error handling

- Search API failure (non-2xx, timeout, rate-limit) → log + fall back to `stubProvider` for that call; `research.output.web = []` with `source:"stub"`. Never fatal.
- No `SEARCH_API_KEY` → engine starts on stub; warning logged once.
- Memory write failure → log, continue; `results.output` still written with `memory_write_error`.
- `SearchMemory` failure → log + treat as empty memory (Phase 0 behavior).
- Per-search `context` timeout (default 8s) and one retry with backoff.

## 5. Testing

- `ResearchProvider` httptest: spin `httptest.Server`, point `httpProvider` at it, assert vendor-JSON → `SearchResult` normalization; assert timeout → fallback to stub.
- Learnings extractor unit: fake `model.LLM` returns canned learnings → assert the produced text carries every required label.
- Research-stage integration: seed `memory.InMemoryService()` + a session event (via `AppendEvent`) containing a "winning hook" → run stage with a fake provider → assert the seeded learning is retrieved and merged into `research.output`.
- Results-stage integration: run with a fake model → assert `AppendEvent` was called with learnings text and `AddSessionToMemory` was invoked; kill the memory service → assert `results.output` still produced.
- E2E smoke (`AUTO_APPROVE=true`, stub provider): two consecutive runs — assert run 2's research retrieves run 1's learnings.

## 6. Config additions

`SEARCH_PROVIDER` (`tavily`|`serper`|`bing`|`gemini`|`stub`; default `stub`), `SEARCH_API_KEY`, `SEARCH_TIMEOUT=8s`, `MEMORY_BACKEND` (`inmemory`|`vertex`|`file`; default `inmemory`), `MEMORY_FILE_PATH=./.gbrain/memory.jsonl`. Document in `.env.example`; never commit keys.

## 7. Definition of done (Phase 3)

- `go build/test/vet ./...` and `golangci-lint run` clean.
- With a real `SEARCH_API_KEY`, a run's `research.output` contains live web snippets; without it, the run still completes on the stub.
- A second run's research stage visibly retrieves the first run's extracted learnings from memory (asserted in E2E + shown in output).
- `ResearchProvider` and the memory backend are interfaces — swapping vendor/backend is a config edit, not a code change.
