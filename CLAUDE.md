# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Example built on **Google's Agent Development Kit for Go** (`google.golang.org/adk/v2`, "adk-go").
- **Module:** `github.com/captain-corgi/go-adk-example`
- **Go version:** 1.26.5 (new — don't assume older stdlib features are unavailable).
- **adk-go import:** the Go import path is `google.golang.org/adk` (the `/v2` is the module-path suffix, not part of the import). After adding the first real import, run `go mod tidy` to drop the `// indirect` marker from `google.golang.org/adk/v2` in `go.mod`.

## Commands

```sh
go build ./...
go test ./...
go vet ./...
golangci-lint run        # config in .golangci.yml (govet, staticcheck, errcheck, ineffassign, unused, gofmt, goimports)
gofmt -w .
goimports -w .
go mod tidy
```

## Environment

Runtime calls models through an **OpenAI-compatible API** (adk-go reaches non-Gemini providers via its LiteLLM integration). The API key is supplied via environment variable — **never hardcode or commit secrets**. Keep keys in a local `.env` (gitignored), not in source.

## Git workflow

- **Default branch is `develop`, not `master`/`main`** — open PRs against `develop`.
- Branch from `develop` as `feature/<topic>` (current: `feature/helloworld`).
- Commit messages use **Conventional Commits**: `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`.
