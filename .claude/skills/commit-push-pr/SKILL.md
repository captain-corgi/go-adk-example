---
name: commit-push-pr
description: Stage the current changes, commit them with a Conventional Commit message, push the current feature branch, and open a pull request against develop. Use when a set of changes is ready to ship. Triggered manually with /commit-push-pr.
disable-model-invocation: true
---

# commit-push-pr

Finalize and ship a set of changes: stage, commit (Conventional Commits), push the feature branch, and open a PR against `develop`.

## Arguments

`$ARGUMENTS` is optional.
- If provided, use it as the basis for the commit subject and PR description (e.g. `/commit-push-pr add hello-world streaming example`).
- If omitted, infer the change from `git status` / `git diff` and confirm with the user before committing.

## Workflow

1. **Verify branch.** Confirm the current branch is `feature/<topic>` and was branched from `develop`; stop unless this holds. If you're on `develop`, `master`, or any other branch, create a `feature/<topic>` branch **from `develop`** first — never commit directly to `develop`, and never branch from `master` or any source other than `develop`.
2. **Review the diff.** Run `git status`, `git diff` (unstaged changes), and `git diff --cached` (staged changes). Briefly summarize what changed for the user. After staging in step 3, re-run `git diff --cached` — the summary and user approval must be based on that exact staged patch, not the unstaged diff.
3. **Stage.** `git add -A`, or stage only the files you both agree on.
4. **Commit.** Write a **Conventional Commit** message:
   - Type prefix: `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`, or `build:`.
   - Imperative mood, subject ≤ 72 chars.
   - Add a body only when the *why* is non-obvious.
5. **Push.** `git push -u origin HEAD`.
6. **Open PR against develop.** `gh pr create --base develop --title "<subject>" --body "<summary>"`. If `gh` is not installed, fall back to printing the GitHub compare URL and tell the user.
7. **Report.** Return the commit hash and the PR URL.

## Constraints

- Never amend, force-push, squash, or rebase without explicit confirmation.
- Never skip hooks (`--no-verify`) or bypass signing unless the user asks.
- Don't push secrets — if you spot an API key, token, or `.env` content, **stop and warn** before committing.
- Confirm the commit message with the user before committing when `$ARGUMENTS` was not supplied.
