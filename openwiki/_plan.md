---
type: Documentation Update Plan
title: Mermaid Diagram Update Plan
description: Plan for adding source-grounded Mermaid diagrams to the repository wiki pages.
tags: [documentation, mermaid]
---

# Plan

- `/openwiki/quickstart.md`: add a high-level user flow from configuration through execution, approval, build, and artifact output; evidence from `cmd/engine/main.go`, `internal/stages`, and existing quickstart claims.
- `/openwiki/architecture/overview.md`: retain the existing request-flow sequence because it is source-grounded and accurate.
- `/openwiki/workflows/pipeline.md`: retain the existing build graph because it represents the workflow control flow.
- `/openwiki/domain/marketing-engine.md`: add a domain transformation flow from idea and brand context to validated landing page; evidence from `brand/`, `internal/brain`, and `internal/build/deliverables/landingpage`.
- `/openwiki/operations/runbook.md`: add a runtime operations flow showing configuration loading, model construction, interactive/automatic sign-off, in-memory artifact save, and memory recording; evidence from `internal/config` and `cmd/engine/main.go`.
- `/openwiki/testing/testing.md`: retain the existing test-validation flow because it covers the documented test layers.
- `/openwiki/integrations/model-and-ci.md`: add a sequence showing application model injection separately from OpenWiki GitHub Actions automation; evidence from `internal/config`, `internal/routing`, and `.github/workflows`.
- `/openwiki/source-map.md`: add an extension flow connecting composition root, contracts/stages, build graph/deliverables, and test infrastructure; evidence from the source map's listed package ownership.

Relationships:
- Quickstart -> navigates to -> architecture, workflows, domain, operations, testing, integrations, source map.
- Configuration -> constructs -> model roles.
- Sequential pipeline -> publishes state to -> sign-off and build.
- Brand context -> shapes -> campaign stages.
- Build graph -> saves -> landing-page artifact.
- Test infrastructure -> validates -> pipeline contracts and workflows.
- GitHub Actions workflow -> updates -> generated wiki.

Remaining questions: none for this documentation-only diagram update; diagrams must remain concise and use Mermaid-safe labels.
