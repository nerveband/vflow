# AGENTS.md

## First Commands

- `go test ./...`
- `go vet ./...`
- `go run ./cmd/vflow schema --validate --format json`
- `go run ./cmd/vflow doctor --format json`
- `go run ./cmd/vflow audit cli --format json`

## Safety Rules

- Never write raw API keys, tokens, or copied `.env` values into this repo.
- Use runtime environment variables or Secret Gate for live provider calls.
- Mutating commands must support `--dry-run` and require `--commit` for writes.
- Live provider calls require explicit `--live`; destructive or costly calls should also use `--commit`.
- Local copied media under `work/` is private and ignored. Do not publish it.

## Output Rules

- Use `--format json` for agent workflows.
- Errors must use `vflow-error/v1` with `code`, `message`, `hint`, `retryable`, and `exit_code`.
- Do not hide warnings in logs; put them in JSON response data or review artifacts.

## NLE Roundtrip Guardrails

- `vflow` owns canonical JSON artifacts.
- NLE files are import/export adapters, not source of truth.
- Every NLE export must include a sidecar mapping source frames to timeline frames.
- Block or review ambiguous effects, speed changes, color grades, plugin effects, nested timelines, and missing sidecar IDs.

## Common Pitfalls

- Do not invent crop boxes from agent suggestions; use approved preset IDs.
- Keep frame numbers canonical; seconds are readable derivatives.
- Prefer source-camera media under project `media/source-4k/` for real fixture tests.
- Do not touch `/Volumes/Shams Drive` from this project.
