# Phase 1 — Swappable Build Design (provisional)

**Goal:** make the deliverable-**GRAPH** and eval-**LOOP** first-class and pluggable, and add **3 verticals** — `social`, `email`, `paidads` — so the build graph fans out to **4 deliverables** (landing page from Phase 0 + these 3), each running its own eval loop, then joined. Builds directly on the [Phase 0 spine](./2026-08-02-phase0-spine-design.md) against the [shared contract](./00-marketing-engine-overview.md).

**Provisional decision (called out):** contract §3.5 names `parallel_worker` for fan-out. `workflow.ParallelWorker` runs *one* wrapped node per slice item and **fails fast** (cancels in-flight siblings) on any non-retryable error — incompatible with Phase 1's partial-success requirement and with each vertical needing its *own* loopagent. For N=4 deliverables known at startup, we instead build **one explicit graph edge per deliverable into a single `JoinNode`**. This honors the §3.5 *shape* (`signoff.output` → N loop-wrapped deliverable nodes → `join` → `build.output`) and uses `workflow.NewJoinNode` directly. `ParallelWorker` remains the scale path when N turns dynamic in a later phase; the seam is the registry, so the swap is local to `internal/build/graph`.

---

## 1. Architecture

```
                       ┌─── build graph (workflow.Workflow) ───┐
signoff.output ──► dispatch ──┬─► landing_page (loopagent) ─┐
                              ├─► social        (loopagent) ─┤
                              ├─► email         (loopagent) ─┼─► build_join (JoinNode) ─► build_output ─► build.output + artifacts
                              └─► paidads       (loopagent) ─┘
```

- `dispatch` reads `signoff.output`, resolves enabled `Deliverable`s from the registry, and seeds each branch's `BuildInput`.
- Each deliverable node is an `agent_node` wrapping that vertical's `loopagent` (drafter + checker). The checker's quality protocol is per-vertical (§2.2).
- `build_join` is a `workflow.NewJoinNode("build_join")`; its output is a `map[string]any` keyed by predecessor name (`landing_page`, `social`, …).
- `build_output` flattens the map into `build.output` and persists one artifact per deliverable via the artifact service.

## 2. Components

### 2.1 `internal/build/deliverables` — registry + loop contract (an ADDITION to the contract)
The contract's `Deliverable` interface (§3.3) is unchanged. We add **one** optional interface so the shared loop builder can pull per-vertical checker logic without redefining `Deliverable`:

```go
type LoopSpec struct {
    DrafterInstruction string            // templated, reads BuildInput
    CheckerInstruction string            // the per-vertical QUALITY PROTOCOL
    OutputSchema       *jsonschema.Schema // vertical-specific artifact shape
    MaxIterations      uint              // 0 => global MAX_LOOP_ITER
}
type Loopable interface {  // Deliverable already satisfies the build contract; Loopable opts into the loop
    Deliverable
    Loop() LoopSpec
}
```

A `registry` (package-level map name→`Deliverable`, populated via `Register`) drives both graph construction and `ENABLED_DELIVERABLES` filtering. The shared `internal/build/loop` builder takes a `Loopable`, assembles `loopagent.New(loopagent.Config{AgentConfig: agent.Config{SubAgents: {drafter, checker}}, MaxIterations: maxIter})`, where `maxIter` is `spec.MaxIterations` when non-zero, otherwise `globalCap` (a bitwise OR combines bits rather than selecting, so it is not used). The builder gives the checker `exitlooptool.New()` so it terminates on pass.

### 2.2 New vertical packages (each implements `Loopable`)
- **`internal/build/deliverables/social`** — `Name()="social"`. Emits threads, long posts, carousels, shorts-hooks. Quality protocol: hook in the first line, on-brand voice (per `{artifact.voice}`), platform length caps, one CTA.
- **`internal/build/deliverables/email`** — `Name()="email"`. Emits welcome flow, nurture, launch. Protocol: subject + preview text present, single CTA, merge-tag safety, ≤N words.
- **`internal/build/deliverables/paidads`** — `Name()="paidads"`. Emits hooks+angles, static creative copy, video-variant scripts. Protocol: hook inside first 3 sec / first 5 words, angle clarity, CTA, per-placement character limits.
- **`landingpage`** (Phase 0) gains a `Loop()` method to stay uniform; its `Build()` body is unchanged.

Each package is unit-testable in isolation via `BuildInput` and the shared loop builder.

### 2.3 `internal/build/graph`
Reads the registry, drops each into its own `agent_node` (each wrapping the vertical's loopagent), wires all as successors of `dispatch` and predecessors of `build_join`, then `build_output`. Adding a 5th vertical (Phase 2) is a new package + one `Register` call — no graph edits.

## 3. Data flow (happy path)

1. `dispatch` reads `signoff.output`, applies `ENABLED_DELIVERABLES`, constructs a `BuildInput{Brief, Plan, Research, Brand}` per vertical from state.
2. All four deliverable loopagents run concurrently (graph fan-out; isolated per node).
3. Each loop draft→check→fixes until the checker calls `exit_loop` (`Actions.Escalate=true`) or `MaxIterations` is hit.
4. `build_join` (`JoinNode`) emits `map[string]any{landing_page:…, social:…, email:…, paidads:…}` once all four predecessors complete.
5. `build_output` writes one artifact per deliverable and sets `build.output` to the joined map.

## 4. Error handling (per-node isolation → partial success)

- **Transient model errors:** each deliverable node carries `NodeConfig{RetryConfig: workflow.DefaultRetryConfig()}` (5 attempts, 2× backoff); retries are independent per node.
- **Non-retryable vertical failure (max-iter, schema, content policy):** the loopagent returns the **best draft as a normal output** tagged `quality:"max_iter_reached"` (Phase 0 pattern) — *not* a Go `error`. Returning a Go error would abort the whole graph (and would fail-fast `ParallelWorker`); we therefore reserve Go errors for truly fatal/unexpected failures only.
- **Join + partial success:** `build_join` waits for every predecessor regardless of per-node quality flags, so one weak vertical never kills siblings. `build_output` sets `build.output.partial=true` and lists `failed:[…]` when any deliverable is `max_iter_reached`/errored; `results` logs these to the gBrain seam as "what died" signals.
- **Disabled set:** an empty/unset `ENABLED_DELIVERABLES` entry is skipped at `dispatch`; `build.output.skipped:[…]` records it.

## 5. Testing

- **Per-vertical units:** `social.Build`, `email.Build`, `paidads.Build` with a fake `model.LLM` returning canned content → assert each emits its required sections (e.g. email has subject+preview+CTA).
- **Loopable contract:** each vertical's `Loop()` returns a `LoopSpec` whose checker instruction references its quality protocol; assert the shared loop exits on iter 2 (checker calls `exit_loop`) and ships best draft with `max_iter_reached` when it never passes.
- **Graph fan-out + join:** with `landingpage`, `social`, `email`, `paidads` all registered and fake models, assert `build.output` is keyed by all 4 names.
- **Sibling isolation:** one vertical's fake model returns a persistent schema error; assert the other 3 still complete and `build.output.partial==true` with the failing name in `failed`.
- **Registry/enablement:** `ENABLED_DELIVERABLES=landing_page,email` → only those two edges exist; the rest appear in `skipped`.

## 6. Config additions

- `ENABLED_DELIVERABLES` — comma-list, default `landing_page,social,email,paidads`. Documented in `.env.example`.
- Reuses Phase 0's `MAX_LOOP_ITER` as the global cap (per-vertical `LoopSpec.MaxIterations` overrides when non-zero). No new model-routing env.

## 7. Definition of done (Phase 1)

- `go build ./...`, `go test ./...`, `go vet ./...`, `golangci-lint run` all clean.
- With a real key + `AUTO_APPROVE=true`, a one-line idea produces **4** deliverable artifacts (landing page + social + email + paidads) from a single run, with gBrain voice/ICP visible in each.
- Adding a hypothetical 5th vertical requires only a new package under `internal/build/deliverables/` + a `Register` call — **zero edits** to `internal/build/graph` or the contract interfaces.
- Every new vertical has a `Build` unit test and a `Loopable` contract test; the partial-success and enablement tests in §5 pass.
- The one contract addition (`Loopable`/`LoopSpec`) is documented in the overview and covered by at least one test per implementer.
