# AGENTS.md

## First Commands

- `go test ./...`
- `go vet ./...`
- `go run ./cmd/vflow schema --validate --format json`
- `go run ./cmd/vflow doctor --format json`
- `go run ./cmd/vflow audit cli --format json`

## Build, Test, and Release Process

- Build the local CLI with `go build -o bin/vflow ./cmd/vflow`.
- Every completed feature, bug fix, workflow change, or user-visible behavior change must include the full finish path before handoff unless the user explicitly says not to release:
  - update `README.md`, bundled skills/docs, schemas, and examples that are affected by the change;
  - run the full local gate below;
  - build and install the local binary on this Mac;
  - commit the change;
  - create the next semver patch tag;
  - push the branch and tag so the GitHub Actions GoReleaser workflow publishes a GitHub Release;
  - verify the published release and run `vflow upgrade --format json --format-error json` on this computer.
- Do not call CLI work done while it only exists as a local dev binary. Done means docs updated, committed, released, and the installed `~/.local/bin/vflow` can upgrade to the released binary.
- Run the full local gate before claiming CLI work is done:
  - `go test ./...`
  - `go vet ./...`
  - `go run ./cmd/vflow schema --validate --format json --format-error json`
  - `go run ./cmd/vflow doctor --format json --format-error json`
  - `go run ./cmd/vflow audit cli --format json --format-error json`
- For release candidates, also run `goreleaser check` before tagging.
- Release binaries must come from GitHub Releases and include platform archives plus `checksums.txt`.
- Release installs and upgrades must verify checksums before replacing a binary.
- Use one release publisher per tag. Prefer the GitHub Actions release workflow after pushing a tag; do not also run local `goreleaser release` for the same tag unless the workflow is disabled or the GitHub release/assets have been removed first.
- If a release workflow fails with GitHub API `422 already_exists`, inspect whether the tag was already published locally and whether the release already has assets with the same names.
- Keep release artifacts out of git except for source, config, docs, and scripts. Do not commit `bin/`, `dist/`, `tmp/`, or private media outputs.

## Local Install and Upgrade Expectations

- This Mac should have `vflow` installed at `~/.local/bin/vflow` unless the user explicitly chooses another install directory.
- After a release, verify the installed binary on this computer:
  - `command -v vflow`
  - `vflow version --format json --format-error json`
  - `vflow upgrade --format json --format-error json`
- The intended user-facing updater is `vflow upgrade`: it should get the latest official binary straight from GitHub Releases, verify `checksums.txt`, back up the existing binary, and install the replacement.
- Agent and CI workflows must preserve structured JSON output and explicit safety semantics. If an upgrade path mutates the local machine, the implementation must make that behavior clear in JSON and tests.
- The installer script is a bootstrap path for machines without `vflow` on `PATH`; once installed, prefer testing the native `vflow upgrade` command.
- Do not regress the updater into a staged-only downloader. A successful committed upgrade must leave `command -v vflow` pointing at the installed release binary.

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
- Prefer copied local fixture media under the project folder for real-media tests.
- Do not reference private source-drive paths or publish private media artifacts.
