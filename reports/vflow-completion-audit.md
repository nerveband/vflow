# vflow Completion Audit

Date: 2026-06-14

This audit checks the active goal against current repo state and command output.

## Proven

- Public repository exists and is public: `https://github.com/nerveband/vflow`.
- Current branch is `main` tracking `origin/main`.
- Unit and integration tests pass with `go test ./...`.
- Static checks pass with `go vet ./...`.
- Command schema validates with `vflow schema --validate --format json`.
- CLI audit passes hardened threshold: score `100`, threshold `85`.
- Repo has `AGENTS.md`, `SKILL.md`, bundled workflow skill, schemas, CI, GoReleaser config, and install script.
- Structured JSON success/error envelopes are implemented and tested.
- Local-first project/media/transcript/cleanup/framing/timeline/render/color/NLE workflows run without API keys.
- Live STT calls succeeded for OpenAI, ElevenLabs, Soniox, AssemblyAI, Deepgram, and Gladia against the ignored synthetic speech fixture; bakeoff wrote `work/live-provider-proof/speech/reports/provider-bakeoff.json`.
- Live STT adapters are implemented for OpenAI, ElevenLabs, Deepgram, AssemblyAI, Gladia, and Soniox; optional providers without runtime keys are skipped explicitly in bakeoff output.
- Gemini Files API upload path is implemented, tested, and live-proven; `qa analyze --upload files` uploaded video, polled to `ACTIVE`, wrote `work/live-provider-proof/gemini/reports/gemini-video-qa.json`, and returned one candidate.
- Live Gemini-backed `color review` completed and wrote `work/live-provider-proof/gemini/reports/color-grade-report.json`.
- Gemini provider responses are sanitized so transient `thoughtSignature` payloads are not emitted in committed CLI reports.
- Copied CAIR-GA fixture probe recognized four copied 3840x2160 source-camera clips under `media/source-4k`.
- Ignored `work/` and `tmp/` proof artifacts were not tracked into the public repo.

## Improved In Continuation

- Config/profile commands now persist YAML config and redact stored secrets.
- Job ledger is durable under `project/jobs/`; committed preview renders write job records.
- Artifact `file:` delivery is atomic and rejects existing files unless `--overwrite --commit` is used.
- Artifact `webhook:<url>` delivery posts a versioned JSON envelope in commit mode.
- `media proxy --commit` and `media samples --commit` now execute ffmpeg and keep agent-readable command plans.
- `render verify` can parse ffprobe JSON and report duration, resolution, codec, audio stream count, and frame count.
- `cleanup review` can write an HTML review artifact.
- NLE exporters now emit structured EDL, FCPXML/Resolve, Premiere XMEML, MLT, and OTIO text plus sidecars with roundtrip segment metadata.
- `nle import` writes neutral `imports/nle-import.json`; `nle diff` classifies safe/review/blocked/unclassified buckets and can write an HTML roundtrip review.
- `nle apply --commit` refuses blocked or unreviewed changes and writes `imports/applied-nle-changes.json` for safe changes.
- `nle accept --commit` writes an explicit accepted-review artifact; `nle apply --commit` can merge accepted needs-review changes from that artifact.
- `color review` writes `reports/color-grade-report.json` without requiring Gemini and can enrich it with live Gemini when runtime credentials are present.
- `project index --path` writes a SQLite/FTS index via `modernc.org/sqlite` and project `reports/provenance.json`; `transcript search --data-source local` returns FTS transcript hits with project IDs and frame ranges.
- `upgrade` checks GitHub release metadata, selects the current OS/arch asset, detects checksum assets, and staged the public `v0.1.1` Darwin arm64 release asset into `tmp/upgrade-proof`.
- Public release `https://github.com/nerveband/vflow/releases/tag/v0.1.1` exists with platform archives and `checksums.txt`.
- `audit cli` now runs a weighted evidence scorecard from `internal/audit` instead of returning a hardcoded scaffold score.

## Not Yet Fully Proven

- NLE roundtrip support is structured and tested for representative artifacts, but it is not yet exhaustively proven against real exported projects from every target editor.
- Editor-specific NLE proof still needs real Resolve/FCP/Premiere/Shotcut/OTIO exported timelines for broader compatibility coverage.

## Current Decision

The CLI implementation is hardened past the 85-point target and all local/provider paths available in this environment were implemented and live-proven. The only remaining compatibility gap is broader real-editor NLE roundtrip fixture coverage.
