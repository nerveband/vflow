# vflow Completion Audit

Date: 2026-06-14

This audit checks the active goal against current repo state and command output.

## Proven

- Public repository exists and is public: `https://github.com/nerveband/vflow`.
- Current branch is `main` tracking `origin/main`.
- Unit and integration tests pass with `go test ./...`.
- Static checks pass with `go vet ./...`.
- Command schema validates with `vflow schema --validate --format json`.
- CLI audit passes current scaffold threshold: score `72`, threshold `65`.
- Repo has `AGENTS.md`, `SKILL.md`, bundled workflow skill, schemas, CI, GoReleaser config, and install script.
- Structured JSON success/error envelopes are implemented and tested.
- Local-first project/media/transcript/cleanup/framing/timeline/render/color/NLE workflows run without API keys.
- Live OpenAI STT call succeeded during proof and wrote canonical transcript words.
- Gemini live calls reached Google but were blocked by an expired runtime API key; errors were structured and redacted.
- Copied CAIR-GA fixture probe recognized four copied 3840x2160 source-camera clips under `media/source-4k`.
- Ignored `work/` and `tmp/` proof artifacts were not tracked into the public repo.

## Improved In Continuation

- Config/profile commands now persist YAML config and redact stored secrets.
- Job ledger is durable under `project/jobs/`; committed preview renders write job records.
- Artifact `file:` delivery is atomic and rejects existing files unless `--overwrite --commit` is used.
- `render verify` can parse ffprobe JSON and report duration, resolution, codec, audio stream count, and frame count.
- `cleanup review` can write an HTML review artifact.
- NLE exporters now emit structured EDL, FCPXML/Resolve, Premiere XMEML, MLT, and OTIO text plus sidecars with roundtrip segment metadata.
- `nle import` writes neutral `imports/nle-import.json`; `nle diff` classifies safe/review/blocked/unclassified buckets and can write an HTML roundtrip review.
- `nle apply --commit` refuses blocked or unreviewed changes and writes `imports/applied-nle-changes.json` for safe changes.
- `color review` writes `reports/color-grade-report.json` without requiring Gemini.

## Not Yet Fully Proven

- NLE roundtrip support is structured and tested for representative artifacts, but it is not yet exhaustively proven against real exported projects from every target editor.
- Accepted-review artifact semantics for needs-review NLE changes are not yet implemented beyond blocking unsafe commit.
- Gemini Files API upload path for large videos is not implemented; current path is inline video and live use is blocked until the expired key is rotated.
- ElevenLabs, Soniox, AssemblyAI, Deepgram, and Gladia live STT adapters are not implemented beyond readiness/config metadata.
- SQLite/FTS indexing from the written plan is not implemented.
- Audit score remains `72`, below the plan’s eventual public-use target of `80+`.

## Current Decision

The goal is materially advanced but not yet complete against the full written plan. Keep the goal active unless the remaining items are explicitly descoped.
