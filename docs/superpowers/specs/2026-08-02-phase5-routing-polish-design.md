# Phase 5 — Real Routing + Polish Design (provisional)

**Goal:** upgrade the Phase 0 routing seam from "one model, two role aliases" to genuine multi-provider routing, harden the human sign-off gate into a rich approve/edit/reject flow, and add telemetry (token usage, per-stage latency, eval-loop iterations, estimated cost). Builds on the [shared contract](./00-marketing-engine-overview.md) §3.7/§3.8 and the [Phase 0 spine](./2026-08-02-phase0-spine-design.md).

**Open dependency (read first):** real routing requires a second provider/endpoint. Whether a second key/endpoint is available is **unknown**. The default design therefore ships with `strong == cheap` (single provider — byte-for-byte Phase 0 behavior) and **auto-activates** distinct routing the moment a second provider is configured via env. No code change is needed to flip the switch.

---

## 1. Architecture

- **`routing.Models` becomes a provider-aware registry.** A `Provider` is a named `{BaseURL, APIKey, factory}`; Phase 0's single `openaimodel.NewModel` call generalizes to one `model.LLM` per `(provider, model)` pair, built lazily and cached. The role→model map (contract §3.7) is resolved at startup: `strong` → `(MODEL_STRONG_PROVIDER, MODEL_STRONG)`, `cheap` → `(MODEL_CHEAP_PROVIDER, MODEL_CHEAP)`. When both pairs resolve to the same provider+model, the registry returns one shared instance — exactly the Phase 0 path.
- **Per-agent model assignment at construction, not via callback swap.** `llmagent.Config.Model` is set to the role-resolved `model.LLM` when each stage agent is built. ADK's `BeforeModelCallback` cannot hot-swap the model instance (it only short-circuits a call by returning an `*model.LLMResponse`), so routing is resolved once at the registry; the callbacks are reserved for **observability and cost accounting**.
- **Shared callback layer.** Every `llmagent` stage gets the same `BeforeModelCallback` (stamp start time + role, derived from `ctx.Agent().Name()`) and `AfterModelCallback` (read `llmResponse.UsageMetadata` for prompt/candidate/total tokens, `llmResponse.ModelVersion` / `CustomMetadata["openai_model"]` for the billed model, compute cost, hand the record to the telemetry collector). This is the canonical ADK hook ("log model responses, collect metrics on token usage").
- **Hardened sign-off** via `workflow` HITL: `workflow.NewRequestInputEvent` carrying a `ResponseSchema` enumerating `{decision: approve|edit|reject, editedPlan?, reason?}`, re-entered through `Workflow.Resume` (ADK validates the payload → `ErrInvalidResumeResponse` surfaces as a retry prompt). `toolconfirmation`'s `adk_request_confirmation` is the fallback approve/deny-only path for clients that prefer the FunctionCall shape.
- **Telemetry** via ADK OTel (`telemetry.New` → `Providers.SetGlobalOtelProviders()`, exporting spans to JSONL through a `SpanProcessor`) **plus** callback-collected metrics aggregated by the `internal/telemetry` collector and merged into `results.output` at run end.

## 2. Components

- **`internal/routing`** — `Provider` registry (`Register`, `LLM(provider, model)`), the `Models` role map (`Strong()`, `Cheap()` → `model.LLM`), `RoutingCallbacks(collector)` constructors for the shared before/after callbacks, and the failover policy. (Also hosts the `Renderers` sibling registry introduced in Phase 4 for the `video` role.)
- **`internal/telemetry`** — `Collector` with `OnAfterModel(record)` accumulating per `{stage, model, tokens, cost, latencyMs}` and per-deliverable eval-loop iteration counts (count of drafter model calls within a build run). `Summary()` emits the JSON block folded into `results.output`. `TELEMETRY_ENABLED=false` swaps in a no-op collector; the collector never panics the run.
- **`internal/stages/signoff`** — upgraded gate: renders `plan.output` as a structured prompt, supports approve / edit (edited plan flows to build) / reject (rejected plan + reason written to memory as a "what died" entry, then ends cleanly). `SIGNOFF_MODE=auto` (and legacy `AUTO_APPROVE=true`) short-circuits to approve with the decision logged.

## 3. Data flow

Pipeline shape is unchanged. Per role: synthesis, checker, and sign-off eval bind `Strong()`; drafter, brief, ideation, **research**, and the Phase 3 learnings extractor bind `Cheap()`. On every model call the `AfterModelCallback` emits one metrics record tagged by stage name. Sign-off now branches three ways; reject terminates before build and records learnings. At results time the collector's summary is written into `results.output` alongside the deliverable reference.

## 4. Error handling

- **Provider failover (routing-layer, not callback):** because callbacks cannot hot-swap the model instance (§1), failover lives in a routing-layer wrapper: `Strong()` returns a model that retries once, then proxies to `Cheap()` on repeated error, logging a `degraded: strong→cheap` marker (judgment quality degrades gracefully; the run still completes). `cheap` has nowhere cheaper to go, so its persistent failure surfaces as a stage `error` per Phase 0 rules. `OnModelErrorCallback` only records the failure for telemetry.
- **Sign-off reject** ends cleanly: no build runs; rejected plan + reason persisted to memory.
- **Telemetry failure** (missing usage metadata, unset rate) is logged and skipped — never breaks the run.
- **Known risk to validate:** the v2.1.0 `openaimodel` multi-turn encoding bug (assistant turns sent as `input_text`) must be re-checked against any newly-wired provider; if it reproduces on multi-turn stages, those stages stay single-turn via the edit-resume pattern. Report-only; no local patch.

## 5. Testing

- **Role resolution:** registry returns the same instance when providers match; distinct instances when `MODEL_STRONG_PROVIDER ≠ MODEL_CHEAP_PROVIDER`.
- **Callbacks:** fake `model.LLM` returning canned `UsageMetadata` → assert the collector recorded expected tokens/cost/latency and the correct model tag.
- **Sign-off:** three paths — approve (resume proceeds), edit (edited plan reaches build), reject (run ends, memory holds the "what died" entry). Plus schema-mismatch → `ErrInvalidResumeResponse` triggers a retry.
- **Failover:** `strong` provider's fake errors twice → callback returns the `cheap` model's response and logs degradation.
- OTel span export is environment-dependent, so it is not asserted in unit tests; collector-summary assertions cover the same ground.

## 6. Config additions

`MODEL_STRONG_PROVIDER`, `MODEL_CHEAP_PROVIDER` (default `openai`); per-provider `PROVIDER_<NAME>_BASE_URL` / `PROVIDER_<NAME>_API_KEY`; optional `MODEL_COST_<provider>_<model>_INPUT_PER_MTPM` / `..._OUTPUT_PER_MTPM`; `TELEMETRY_ENABLED=true`; `TELEMETRY_EXPORT_PATH`; `SIGNOFF_MODE=interactive` (values: `interactive|auto`; `AUTO_APPROVE=true` retained as a back-compat alias for `auto`). Document in `.env.example`.

## 7. Definition of done (Phase 5)

- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all clean.
- With a single provider configured, behavior matches Phase 0.
- With a second provider configured, synthesis/checker/sign-off-eval visibly run on `strong` while brief/ideation/drafter run on `cheap`, visible in the telemetry summary.
- A run's `results.output` contains a telemetry block: per-stage token usage + latency, per-deliverable eval-loop iteration count, total estimated cost.
- Sign-off supports approve/edit/reject; reject leaves a "what died" memory entry and exits cleanly.
- `strong` provider failure degrades to `cheap` with a logged marker rather than aborting.
