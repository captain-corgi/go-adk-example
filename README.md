# Marketing Engine

An example built on Google's Agent Development Kit for Go (`google.golang.org/adk/v2`):
one raw marketing idea in → a shipped landing-page campaign out. Phase 0
implements the full spine end-to-end on adk-go.

## Run the Marketing Engine

```sh
go run ./cmd/engine
```

Set `OPENAI_API_KEY` / `OPENAI_BASE_URL` / `OPENAI_MODEL` (and optionally
`MODEL_STRONG`, `MODEL_CHEAP`, `AUTO_APPROVE=true`, `MAX_LOOP_ITER`, `GBRAIN_DIR`)
in a gitignored `.env`. Submit a one-line idea at the prompt; the engine ships a
landing-page Markdown artifact and prints its location.
