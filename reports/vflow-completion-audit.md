# vflow Completion Audit

Date: 2026-06-16

This audit checks the active goal against current repo state and command output.

## Proven

- Public repository exists and is public: `https://github.com/nerveband/vflow`.
- Current branch is `codex/vflow-sync-hardening` tracking `origin/codex/vflow-sync-hardening`.
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
- Committed Gemini QA reports now use the versioned vflow wrapper `vflow-gemini-video-qa/v1`, preserving vflow metadata while nesting raw Gemini output under `provider_response`.
- Command contract registry now includes all implemented plan-listed top-level/framing command surfaces that were previously missing from schema output: `feedback`, `framing propose`, and `framing review`.
- NLE self-roundtrip fixture coverage now parses vflow exports back from EDL, FCPXML/Resolve, Premiere XMEML, MLT, and OTIO; all preserve `seg_A` identity for `clip_trim` and classify without unclassified changes.
- NLE sidecars now have an explicit `schemas/nle-sidecar.schema.json` contract, schema validation includes it, and `nle export` rejects unsupported target typos instead of silently emitting a generic sidecar.
- `nle diff` blocks identity-sensitive NLE changes that lack vflow segment IDs as `missing_sidecar`; raw editor EDL events without `* VFLOW-SEGMENT-ID` cannot be safe-merged.
- FCPXML/Premiere XML marker values count as segment identity only when they explicitly contain `vflow:segment-id=...`; plain editor marker labels now trigger the missing-sidecar guardrail.
- FCPXML retime/media-replacement edits and OTIO time-warp style effects are parsed into needs-review buckets while preserving vflow segment identity.
- `doctor --local` reports NLE targets, import formats, sidecar export support, blocked roundtrip change types, Resolve package handling, and the remaining real-editor fixture proof gap; CI uses the local doctor gate.
- The copied `references/Executive Directors.drp` fixture was inspected as local JSON switcher/project state, not a timeline interchange export; `nle import` now detects `.drp`/`.dra`/`.drt` and returns a structured `NLE_IMPORT_PARSE_FAILED` with the actionable instruction to export FCPXML, EDL, or OTIO from Resolve.
- Copied CAIR-GA fixture probe recognized four copied 3840x2160 source-camera clips under `media/source-4k`.
- Actual CAIR-GA 30-second CLI render wrote `work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/renders/cair-ga-actual-30s.mp4` from copied source-camera media and verified as 1920x1080 H.264/AAC.
- Ignored `work/` and `tmp/` proof artifacts were not tracked into the public repo.

## Improved In Continuation

- Config/profile commands now persist YAML config and redact stored secrets.
- Job ledger is durable under `project/jobs/`; committed preview renders write job records.
- Artifact `file:` delivery is atomic and rejects existing files unless `--overwrite --commit` is used.
- Artifact `webhook:<url>` delivery posts a versioned JSON envelope in commit mode.
- `media proxy --commit` and `media samples --commit` now execute ffmpeg and keep agent-readable command plans.
- `render preview` supports `--start-seconds` and `--output`, so agents can cut a named clip from a specific source offset without overwriting `rough-preview.mp4`.
- `render verify` can parse ffprobe JSON and report duration, resolution, codec, audio stream count, and frame count.
- `cleanup review` can write an HTML review artifact.
- NLE exporters now emit structured EDL, FCPXML/Resolve, Premiere XMEML, MLT, and OTIO text plus sidecars with roundtrip segment metadata.
- `nle import` writes neutral `imports/nle-import.json`; `nle diff` classifies safe/review/blocked/unclassified buckets and can write an HTML roundtrip review.
- `nle apply --commit` refuses blocked or unreviewed changes and writes `imports/applied-nle-changes.json` for safe changes.
- `nle accept --commit` writes an explicit accepted-review artifact; `nle apply --commit` can merge accepted needs-review changes from that artifact.
- `color review` writes `reports/color-grade-report.json` without requiring Gemini and can enrich it with live Gemini when runtime credentials are present.
- `project index --path` writes a SQLite/FTS index via `modernc.org/sqlite` and project `reports/provenance.json`; `transcript search --data-source local` returns FTS transcript hits with project IDs and frame ranges.
- `upgrade` checks GitHub release metadata, selects the current OS/arch asset, detects checksum assets, and staged the public `v0.1.2` Darwin arm64 release asset into `tmp/upgrade-proof-v0.1.2`.
- Public release `https://github.com/nerveband/vflow/releases/tag/v0.1.2` exists with platform archives and `checksums.txt`.
- `audit cli` now runs a weighted evidence scorecard from `internal/audit` instead of returning a hardcoded scaffold score.

## Not Yet Fully Proven

- NLE roundtrip support is structured and tested for vflow-generated interchange artifacts across every target adapter, plus representative FCPXML editor-style changes. It is not yet exhaustively proven against real exported projects from every target editor.
- Editor-specific NLE proof still needs real Resolve/FCP/Premiere/Shotcut/OTIO exported timelines for compatibility beyond vflow-generated fixtures.

## Current Decision

The CLI implementation is hardened past the 85-point target and all local/provider paths available in this environment were implemented and live-proven. The only remaining compatibility gap is broader real-editor NLE roundtrip fixture coverage.

## 2026-06-15 Clip Sync Hardening Addendum

Proven:

- `vflow-media-sync-map/v1` is implemented and tested with explicit transcript, reference-source, and source-camera time mapping.
- Audio waveform sync helpers are implemented in Go: mono 16k PCM ffmpeg extraction planning, RMS envelopes, normalized cross-correlation, confidence scoring, low-confidence validation warnings, drift helper, and waveform proof command planning.
- Storage-aware `media extract-ranges` computes source seconds from transcript ranges through the sync map, estimates byte use, and extracts only requested local source-camera ranges with optional audio mapping, `-dn`, stripped metadata/chapters, H.264/AAC, `yuv420p`, and faststart.
- `cut create` can write a transcript cut with resolved source/reference/transcript seconds.
- `render transcript-cut --sync-map` resolves transcript-relative segments before ffmpeg planning.
- `render verify-transcript` writes a local transcript proof report.
- NLE sidecars carry `sync_map_ref` and optional source/reference/transcript frame provenance fields.

Group 4 proof:

- Source camera files were read from `/Volumes/Shams Drive/CAIR-GA 10 yr/Group 4 Current Board/Camera Source Files` as inputs only; all writes stayed under ignored `work/test-projects/cair-ga-group-4-current-board-social-30s`.
- Known alignment was confirmed by command output: transcript `34:19` (`2059s`) maps to 12mm/9mm `40:15` (`2415s`) and 7mm `40:32` (`2432s`).
- `media extract-ranges --commit` wrote only the needed local 30s range to `media/sync-ranges/group4_known_cta_30s-group4_7mm.mp4`.
- `render transcript-cut --sync-map --commit` wrote `renders/group4-sync-proof-30s.mp4`.
- `render verify` returned `status: valid`, `1920x1080`, duration `30.03`, H.264, one audio stream, 720 frames.
- `OPENAI_API_KEY` was present; live OpenAI STT proof against the proof render wrote 82 words under `live-transcript-proof/transcript/`.

Verification:

- `go test ./...`: pass.
- `go vet ./...`: pass.
- `go run ./cmd/vflow schema --validate --format json --format-error json`: pass.
- `go run ./cmd/vflow audit cli --format json --format-error json`: pass, score `100/100`.

## 2026-06-15 Multi-Angle Sync Cut Addendum

Additional proof:

- Created `decisions/group4-sync-multiangle-ranges.json` with three transcript-selected ranges across 12mm, 9mm, and 7mm.
- `media extract-ranges --commit` wrote local synced range clips under `media/sync-ranges-multiangle/` and `calibration/source-range-manifest-multiangle.json`.
- `cut create --sync-map --commit` wrote `decisions/group4-sync-multiangle-cut.json`.
- `render transcript-cut --sync-map --commit` wrote `renders/group4-sync-multiangle-social-30s.mp4`.
- `render verify` returned `status: valid`, `1920x1080`, duration `30.03`, H.264, one audio stream, and 720 frames.
- Generated a corrected natural LUT at `calibration/group4-natural-contrast-rfast.cube`; `color apply --commit` wrote `renders/group4-sync-multiangle-social-30s-graded-natural-v2.mp4`.
- The graded render verified as `status: valid`, `1920x1080`, duration `30.03`, H.264, one audio stream, and 720 frames.
- Visual comparison proof: `reports/frames/sync-multiangle-ungraded-vs-natural-graded-v2-contact-sheet.png`.
- `render verify-transcript --commit` wrote `reports/group4-sync-multiangle-transcript-proof.json`.
- Live OpenAI STT proof wrote 73 words to `live-transcript-proof-sync-multiangle/transcript/openai-transcription.json`; transcript text matched the intended three-part summary cut.
- `render verify` now accepts `--project` and resolves relative render paths under the project, matching the rest of the render workflow.
- The framing contract compiler slice now writes deterministic `decisions/framing-lane.json` and `review/review-queue.json` from approved presets, speaker map, policy, and word-frame artifacts while preserving dry-run-by-default behavior.
- Framing validation rejects diarization-label preset IDs/labels, source-bound violations, unknown speaker-map presets, invalid word frame ranges, low-confidence fallback cases, overlap wide fallback cases, minimum-dwell review cases, and wide-reset insertion cases.

Provider status:

- `OPENAI_API_KEY` was present and live STT succeeded.
- `GEMINI_API_KEY` was present; after key rotation and Files API auth-path hardening, live Gemini inline and Files API QA succeeded with `gemini-3.5-flash`, and live Gemini color review succeeded on the graded render.

Final verification after framing and render-verify patches:

- `go test ./...`: pass.
- `go vet ./...`: pass.
- `go run ./cmd/vflow schema --validate --format json --format-error json`: pass, command count `63`, schema count `21`.
- `go run ./cmd/vflow doctor --format json --format-error json`: pass; ffmpeg, ffprobe, `OPENAI_API_KEY`, `GEMINI_API_KEY`, and the configured STT provider env vars detected by boolean presence only.
- `go run ./cmd/vflow audit cli --format json --format-error json`: pass, score `100/100`.
