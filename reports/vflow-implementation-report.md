# vflow Implementation Report

Date: 2026-06-14

## Implemented

- Go/Cobra CLI with structured `vflow-response/v1` and `vflow-error/v1` envelopes.
- Schema and agent introspection: `schema --validate`, `agent-context`, `skill-path`, `doctor`, `audit cli`.
- Commit-gated project/media/transcript/cleanup/framing/timeline/render/QA/color/NLE/artifact commands.
- YAML-backed `config` and `profile` commands using `VFLOW_CONFIG_PATH` for isolated tests and `~/.vflow/config.yaml` by default.
- Durable JSON job ledger under `project/jobs/` with `jobs list/get/resume`; committed preview renders now write job records.
- SQLite/FTS project index using `modernc.org/sqlite`: `project index --path` writes `~/.vflow/index.sqlite` or `$VFLOW_INDEX_PATH`, plus project `reports/provenance.json`; `transcript search --data-source local` reads the FTS index.
- Atomic file artifact delivery with overwrite gating.
- Artifact delivery now supports `stdout`, atomic `file:<path>`, and committed `webhook:<url>` POST delivery.
- Live `ffprobe` source review, ffmpeg preview renders with configurable start/output, transcript-selected multi-segment renders, ffmpeg LUT renders, render verification, and NLE sidecars.
- `media proxy --commit` and `media samples --commit` now execute ffmpeg with configurable binary paths and keep dry-run JSON plans.
- Render verification parses ffprobe JSON/evidence for duration, resolution, codec, audio streams, and frame count.
- Live OpenAI STT adapter using `OPENAI_API_KEY` and `/v1/audio/transcriptions`; secrets are env-only.
- Live STT adapters for OpenAI, ElevenLabs, Deepgram, AssemblyAI, Gladia, and Soniox with provider sidecars and canonical frame-word normalization.
- Transcript bakeoff can run live providers with `--live --commit`, records completed/skipped/failed provider status, and writes `reports/provider-bakeoff.json`.
- Gemini QA/color hooks using `GEMINI_API_KEY`, `GOOGLE_API_KEY`, or `GOOGLE_GENERATIVE_AI_API_KEY` with `x-goog-api-key`; provider errors are compact and redacted.
- Gemini QA now supports the Files API resumable upload path, polls uploaded files until `ACTIVE`, and sanitizes transient `thoughtSignature` payloads from CLI/report JSON.
- NLE export/import/diff/apply surfaces for FCPXML, EDL, OTIO, MLT, Resolve alias, Premiere XMEML, and sidecars, with roundtrip segment metadata.
- NLE import parses supported raw timelines into neutral change records, diff classifies safe/review/blocked buckets, and guarded apply writes `imports/applied-nle-changes.json` only when changes are safe to commit.
- `nle accept` writes `imports/accepted-nle-changes.json` artifacts so needs-review changes can be applied only after explicit review acceptance.
- Cleanup review and NLE diff can deliver HTML review artifacts.
- `framing calibrate` starts a managed localhost crop/zoom/reframe calibration session with embedded HTML/CSS/JS, structured session JSON, project-scoped media serving, status persistence under `tmp/sessions/`, API validation, commit-gated writes, and programmatic shutdown. `framing crop`, `framing zoom`, `framing reframe`, `framing frame`, `framing crop-calibrate`, `framing zoom-calibrate`, and `framing preset-calibrate` are aliases for agent discoverability.
- Agent-facing synonym commands now cover common intent terms across project, media, transcript, cleanup, framing, timeline, render, NLE, and artifact workflows while preserving canonical command names in output.
- README and bundled skill docs now explain what `vflow` does for agents, what guarantees it provides, when not to use it, current alias vocabulary, and future feature boundaries.
- Color review writes `reports/color-grade-report.json` without requiring live Gemini, and live Gemini can enrich it when credentials work.
- Public-repo support files: `AGENTS.md`, root `SKILL.md`, bundled workflow skill, schemas, CI, release workflow, GoReleaser config, install script, and research notes.
- `upgrade` now checks GitHub release metadata, selects the current OS/arch asset, detects checksum assets, and can stage a release asset into a cache with `--commit`.
- Public release `v0.1.2` is published with GoReleaser platform archives and `checksums.txt`.
- `audit cli` is backed by `internal/audit` evidence checks instead of a hardcoded score.

## Verification Commands

```bash
go test ./...
make test
make lint
make schema-validate
make doctor
make audit
```

Current results:

- `go test ./...` passed.
- `make test` passed.
- `make lint` / `go vet ./...` passed.
- `schema --validate` returned `status: valid`; current command count is higher because agent-facing aliases are included in the registry.
- `doctor` found `ffmpeg`, `ffprobe`, and `python3`.
- `audit cli` returned score `100` with pass threshold `85`.

Continuation verification also passed:

```bash
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json
go run ./cmd/vflow audit cli --format json
go run ./cmd/vflow doctor --format json
```

Framing calibrator proof commands:

```bash
go test ./internal/framing/session ./internal/cli
go run ./cmd/vflow framing calibrate --project fixtures/project/basic --listen 127.0.0.1:0 --open=false --wait=false --session-timeout 30s --format json --format-error json
go run ./cmd/vflow schema --validate --format json --format-error json
```

The calibrator returns `session_id`, `url`, `health_url`, `status_url`, `shutdown_url`, artifact paths, `port`, `pid`, timeout, and shutdown-token presence. Mutating API writes remain blocked unless the session was started with `--commit` and the API request carries commit intent.

Hardening verification on 2026-06-14:

```bash
go test ./...
go vet ./...
source ~/.config/vflow/secrets.zsh
go run ./cmd/vflow auth doctor --format json --format-error json
go run ./cmd/vflow schema --validate --format json
go run ./cmd/vflow doctor --format json
go run ./cmd/vflow audit cli --format json
go run ./cmd/vflow upgrade --format json --timeout 30s
```

Results:

- `go test ./...` passed across all packages.
- `go vet ./...` passed.
- `auth doctor` returned redacted `env_present: true` for `OPENAI_API_KEY`, `ELEVENLABS_API_KEY`, `SONIOX_API_KEY`, `ASSEMBLYAI_API_KEY`, `DEEPGRAM_API_KEY`, `GLADIA_API_KEY`, `GEMINI_API_KEY`, `HF_TOKEN`, and `ANTHROPIC_API_KEY`.
- `schema --validate` returned `status: valid`; current command count is higher because agent-facing aliases are included in the registry.
- `doctor` found `ffmpeg`, `ffprobe`, and `python3`.
- `audit cli` returned `score: 100`, `threshold: 85`, `status: pass`.
- Release workflow published `v0.1.2`; `upgrade --commit` staged `vflow_0.1.2_darwin_arm64.tar.gz` from the public release into `tmp/upgrade-proof-v0.1.2`.

Additional proof commands:

```bash
VFLOW_CONFIG_PATH=tmp/continuation-proof/config.yaml go run ./cmd/vflow profile set --name cont --provider elevenlabs --api-key-env ELEVENLABS_API_KEY --commit --format json
VFLOW_CONFIG_PATH=tmp/continuation-proof/config.yaml go run ./cmd/vflow config set-defaults --project-root ./work --commit --format json
go run ./cmd/vflow render verify --render rough-preview.mp4 --ffprobe-json fixtures/media/tiny/ffprobe.json --expected-width 1920 --expected-height 1080 --expected-duration 12.345 --format json
go run ./cmd/vflow artifacts deliver --input tmp/continuation-proof/project/reports-source.json --deliver file:tmp/continuation-proof/project/reports-copy.json --commit --overwrite --format json
go run ./cmd/vflow cleanup review --project tmp/continuation-proof/project --deliver file:tmp/continuation-proof/project/review/cleanup-review.html --commit --format json
go run ./cmd/vflow nle import --project tmp/continuation-proof/project --input tmp/continuation-proof/project/timeline.fcpxml --commit --format json
go run ./cmd/vflow nle diff --project tmp/continuation-proof/project --import tmp/continuation-proof/project/imports/nle-import.json --deliver file:tmp/continuation-proof/project/review/roundtrip-review.html --format json
go run ./cmd/vflow nle diff --project fixtures/project/basic --import fixtures/nle/roundtrip.fcpxml --format json
go run ./cmd/vflow color review --project tmp/continuation-proof/project --input tmp/continuation-proof/project/renders/rough-preview.mp4 --commit --format json
VFLOW_INDEX_PATH=tmp/index-proof/index.sqlite go run ./cmd/vflow project index --path tmp/continuation-proof/project --commit --format json
VFLOW_INDEX_PATH=tmp/index-proof/index.sqlite go run ./cmd/vflow transcript search --project tmp/continuation-proof/project --query sample --data-source local --limit 5 --format json
```

Results:

- Config/profile writes persisted and `config inspect` stayed redacted.
- Render verification returned `status: valid`, `width: 1920`, `height: 1080`, `audio_streams: 1`, and `frame_count: 370`.
- Artifact delivery wrote with `status: delivered`.
- Cleanup review wrote `review/cleanup-review.html`.
- NLE import wrote `imports/nle-import.json`; NLE diff wrote `review/roundtrip-review.html`.
- Fixture NLE diff classified `clip_trim`, `marker_note`, and `audio_level` as safe, `crop_change` and `title_card` as needs-review, `color_grade` as blocked, and `unclassified: []`.
- Color review wrote `reports/color-grade-report.json`.
- SQLite project index wrote `tmp/index-proof/index.sqlite` and fixture `reports/provenance.json`; local FTS transcript search returned project ID, word IDs, and frame ranges.

## Synthetic Live Proof

The synthetic demo source was generated under ignored `tmp/` with real ffmpeg:

```bash
ffmpeg -y -hide_banner -loglevel error \
  -f lavfi -i testsrc2=size=1280x720:rate=30 \
  -f lavfi -i sine=frequency=1000:sample_rate=48000 \
  -t 3 -pix_fmt yuv420p -c:v libx264 -preset veryfast -c:a aac \
  tmp/live-demo-source.mp4
```

Commands run with `--commit`:

```bash
go run ./cmd/vflow project init --path tmp/live-demo --id live_demo --commit --format json
go run ./cmd/vflow media ingest --project tmp/live-demo --source tmp/source.mp4 --copy --commit --format json
go run ./cmd/vflow media probe --project tmp/live-demo --source tmp/live-demo/media/source.mp4 --commit --format json
go run ./cmd/vflow transcript create --project tmp/live-demo --provider openai --source tmp/live-demo/media/source.mp4 --model gpt-4o-transcribe --rate 30/1 --live --commit --format json
go run ./cmd/vflow transcript align --project tmp/live-demo --commit --format json
go run ./cmd/vflow transcript search --project tmp/live-demo --query beep --format json
go run ./cmd/vflow cleanup apply --project tmp/live-demo --input tmp/live-demo/decisions/delete_segments.json --rate 30/1 --commit --format json
go run ./cmd/vflow cleanup suggest --project tmp/live-demo --commit --format json
go run ./cmd/vflow cleanup review --project tmp/live-demo --format json
go run ./cmd/vflow framing preset import --project tmp/live-demo --input tmp/live-demo/calibration/framing-presets-input.json --commit --format json
go run ./cmd/vflow framing map-speakers --project tmp/live-demo --commit --format json
go run ./cmd/vflow framing propose --project tmp/live-demo --commit --format json
go run ./cmd/vflow framing compile --project tmp/live-demo --commit --format json
go run ./cmd/vflow framing review --project tmp/live-demo --format json
go run ./cmd/vflow timeline compile --project tmp/live-demo --duration-frames 90 --commit --format json
go run ./cmd/vflow timeline verify --project tmp/live-demo --format json
go run ./cmd/vflow render preview --project tmp/live-demo --source tmp/live-demo/media/source.mp4 --duration-seconds 2 --commit --format json
go run ./cmd/vflow render verify --input tmp/live-demo/renders/rough-preview.mp4 --format json
go run ./cmd/vflow color apply --input tmp/live-demo/renders/rough-preview.mp4 --lut fixtures/color/basic.cube --deliver file:tmp/live-demo/renders/rough-preview-graded.mp4 --commit --format json
go run ./cmd/vflow color export-lut --input fixtures/color/basic.cube --output tmp/live-demo/exports/basic.cube --commit --format json
```

Proof highlights:

- OpenAI live STT succeeded and wrote `tmp/live-demo/transcript/words.json`.
- Preview render verified as H.264, 1920x1080, 2.000000 seconds.
- LUT render verified as H.264, 1920x1080, 2.000000 seconds.
- Timeline compiler regression fixed: first kept segment now maps timeline frame out to `30`, not `0`.
- Render plan regression fixed: one-second previews now use `afade=t=out:st=0.97:d=0.03`.

NLE proof commands:

```bash
for target in fcpxml edl otio mlt resolve premiere sidecar; do
  go run ./cmd/vflow nle export --project tmp/live-demo --target "$target" --deliver "file:tmp/live-demo/exports/timeline.$target" --commit --format json
done
go run ./cmd/vflow nle import --input tmp/live-demo/exports/timeline.fcpxml --format json
go run ./cmd/vflow nle diff --import tmp/live-demo/exports/timeline.fcpxml --format json
go run ./cmd/vflow nle import --project tmp/live-demo --input tmp/live-demo/exports/timeline.fcpxml --commit --format json
go run ./cmd/vflow nle diff --project tmp/live-demo --import tmp/live-demo/imports/nle-import.json --format json
go run ./cmd/vflow nle apply --project tmp/live-demo --input tmp/live-demo/imports/nle-import.json --commit --format json
```

NLE proof result: all seven exports wrote sidecars with two compiled segments; FCPXML import/diff/apply parsed and applied safe roundtrip changes.

## Live Gemini Result

Live Gemini calls completed with the runtime env key, including the Files API upload path:

```bash
source ~/.config/vflow/secrets.zsh
go run ./cmd/vflow qa doctor --provider gemini --model gemini-3.5-flash --live --commit --timeout 3m --format json --format-error json
go run ./cmd/vflow qa analyze --project work/live-provider-proof/gemini --render work/live-provider-proof/gemini/renders/rough-preview.mp4 --provider gemini --model gemini-3.5-flash --upload files --live --commit --timeout 5m --format json --format-error json
go run ./cmd/vflow color review --project work/live-provider-proof/gemini --input work/live-provider-proof/gemini/renders/rough-preview.mp4 --provider gemini --model gemini-3.5-flash --live --commit --timeout 5m --format json --format-error json
```

Results:

- `qa doctor` returned `ok: true`, selected `gemini-3.5-flash`, confirmed the model was available, and listed 50 available models.
- `qa analyze --upload files` uploaded `work/live-provider-proof/gemini/renders/rough-preview.mp4`, polled the uploaded file to `ACTIVE`, wrote `work/live-provider-proof/gemini/reports/gemini-video-qa.json`, and returned one candidate.
- `color review` wrote `work/live-provider-proof/gemini/reports/color-grade-report.json` with one Gemini candidate.
- Both committed Gemini reports were sanitized; `thoughtSignature` was absent from the saved JSON.

## Live STT Provider Result

Live provider bakeoff was run against a synthetic speech fixture under ignored `work/live-provider-proof/`:

```bash
source ~/.config/vflow/secrets.zsh
say -o work/live-provider-proof/speech/media/vflow-speech.aiff "Hello from vflow. This is a live speech recognition provider test."
ffmpeg -y -hide_banner -loglevel error -i work/live-provider-proof/speech/media/vflow-speech.aiff -ar 16000 -ac 1 -c:a pcm_s16le work/live-provider-proof/speech/media/vflow-speech.wav
go run ./cmd/vflow transcript bakeoff --project work/live-provider-proof/speech --source work/live-provider-proof/speech/media/vflow-speech.wav --providers openai,elevenlabs,soniox,assemblyai,deepgram,gladia,local --live --commit --timeout 20m --format json --format-error json
```

Results:

- OpenAI completed with `model: gpt-4o-transcribe`, `word_count: 11`.
- ElevenLabs completed with `model: scribe_v2`, `word_count: 11`.
- Soniox completed with `model: stt-async-v5`, `word_count: 20`, job `03e86368-d235-4ed7-a9dc-183f50c76baf`.
- AssemblyAI completed with `model: default`, `word_count: 11`, job `65243956-f111-4aaf-b7aa-773c3d8d9420`.
- Deepgram completed with `model: nova-3`, `word_count: 11`, job `019ec7b6-ffdd-74a0-9e2d-805867f14050`.
- Gladia completed with `model: pre-recorded-v2`, `word_count: 22`, job `49ebb28a-1beb-4408-b0ea-3d0672ae8f14`.
- Local remained `local_import_only`.
- Bakeoff wrote `work/live-provider-proof/speech/reports/provider-bakeoff.json`.

## Live Media Commit Result

Actual ffmpeg proxy and contact-sheet commands were run against a tiny copied fixture under ignored `work/live-smoke/`:

```bash
go run ./cmd/vflow media proxy --project work/live-smoke/media-render --source work/live-smoke/media-render/media/source.mp4 --commit --overwrite --format json
go run ./cmd/vflow media samples --project work/live-smoke/media-render --source work/live-smoke/media-render/media/source.mp4 --count 6 --deliver file:work/live-smoke/media-render/reports/contact-sheet.jpg --commit --overwrite --format json
```

Results:

- Proxy render wrote `work/live-smoke/media-render/media/proxy.mp4`.
- Contact sheet wrote `work/live-smoke/media-render/reports/contact-sheet.jpg`.

## Public Release And Upgrade Proof

Release:

```text
https://github.com/nerveband/vflow/releases/tag/v0.1.2
```

Proof commands:

```bash
gh release view v0.1.2 --repo nerveband/vflow --json tagName,url,assets,publishedAt,isDraft,isPrerelease
go run ./cmd/vflow upgrade --commit --cache-dir tmp/upgrade-proof-v0.1.2 --timeout 2m --format json --format-error json
```

Results:

- Release `v0.1.2` is public, not draft, not prerelease.
- Assets include `checksums.txt`, Darwin/Linux tarballs, and Windows zip archives for amd64 and arm64.
- `upgrade --commit` returned `status: staged`, `latest_version: v0.1.2`, checksum asset `checksums.txt`, and staged path `tmp/upgrade-proof-v0.1.2/v0.1.2/vflow_0.1.2_darwin_arm64.tar.gz`.

## Local Fixture Proof

Real-media proof was run against ignored, project-local fixture folders using copied source media only. The public repository does not include private media, provider outputs, source-drive paths, or client-specific transcript content.

Fixture proof covered:

- `media probe` over multiple copied 3840x2160 H.264 source-camera clips.
- Transcript import/alignment into canonical word artifacts.
- Timeline compilation from frame-anchored decisions.
- One-second and 30-second preview renders verified as H.264/AAC at 1920x1080.
- Transcript-selected multi-segment render planning and verification.
- LUT application and render verification.
- FCPXML export/import/diff/apply with sidecar identity preservation.
- SQLite/FTS project indexing and transcript search.

## Remaining Work

- Broaden NLE writer/parser fixtures against real Resolve, Final Cut Pro, Premiere, Shotcut/MLT, and OTIO roundtrips; current coverage is structured and tested but not exhaustive for every editor feature.

## 2026-06-15 Clip Sync Hardening Proof

Implemented:

- Canonical `vflow-media-sync-map/v1` contract in `internal/syncmap` with transcript-to-reference and per-source offset math.
- Pure-Go 16kHz PCM RMS-envelope normalization and normalized cross-correlation with confidence scoring.
- FFmpeg extraction/waveform proof command planning using mono 16k PCM, optional audio mapping, `-dn`, metadata/chapter stripping, H.264/AAC, `yuv420p`, and faststart for extracted ranges.
- CLI surfaces: `media sync`, `transcript sync`, `media extract-ranges`, `cut create`, `render verify-transcript`, plus `render transcript-cut --sync-map`.
- NLE sidecar sync-map provenance fields for source/reference/transcript frame roundtrips.
- Schemas: `media-sync-map.schema.json`, `source-range-manifest.schema.json`, `transcript-proof.schema.json`.

Verification commands:

```bash
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

Results:

- `go test ./...`: pass.
- `go vet ./...`: pass.
- `schema --validate`: pass, command count `60`, schema count `17`.
- `doctor`: pass; ffmpeg and ffprobe available, `OPENAI_API_KEY` present.
- `audit cli`: pass, score `100/100`.

Local sync proof:

- Known transcript/reference/source offsets were resolved through `vflow-media-sync-map/v1`.
- `media extract-ranges --commit` extracted only needed project-local source ranges.
- `cut create --commit` wrote transcript cut decisions with sync-map provenance.
- `render transcript-cut --sync-map --commit` rendered synced 30-second clips.
- `render verify` returned valid H.264/AAC, 1920x1080, 30.03 seconds, one audio stream, and expected frame counts.
- `render verify-transcript --commit` wrote transcript proof reports.
- Live STT proof was run against an isolated copied render and wrote provider output under ignored local test folders.
- Color/LUT proof rendered and verified a graded local fixture clip.

## 2026-06-15 Gemini Key Diagnostic

Implemented:

- `qa doctor`, `qa analyze`, and `color review` accept `--key-env` so multiple Gemini candidate keys can be tested by environment variable name without writing raw keys into the repo, command history, or reports.
- Gemini model normalization now accepts the existing `fast`/`deep` aliases, SDK-documented `stable` (`gemini-2.5-flash`) and `video` (`gemini-3-flash-preview`) aliases, `models/gemini-*` IDs, and explicit `gemini-*` model names.
- The direct REST adapter still matches the official GenAI SDK flow: API-key client, `models.generateContent`, Files API upload for videos, then `file_data` generation.

Proof commands:

```bash
go run ./cmd/vflow qa doctor --provider gemini --key-env GEMINI_API_KEY --model gemini-2.5-flash --live --format json --format-error json
cd work/tmp-gemini-sdk-check && go run .
go run ./cmd/vflow qa doctor --provider gemini --model gemini-3.5-flash --live --format json --format-error json
go run ./cmd/vflow qa analyze --project tmp/video-proof --render tmp/video-proof/renders/graded-proof.mp4 --provider gemini --model gemini-3.5-flash --upload inline --live --commit --timeout 5m --format json --format-error json
go run ./cmd/vflow color review --project tmp/video-proof --input tmp/video-proof/renders/graded-proof.mp4 --provider gemini --model gemini-3.5-flash --live --commit --timeout 5m --format json --format-error json
```

Result:

- Both the vflow REST path and the official Go SDK path reached Google and returned consistent provider responses for the configured runtime key.
- To test multiple candidate keys safely, export each key under a distinct variable and run `qa doctor --key-env` for each variable name.
- After rotating/restarting Codex, `qa doctor` passed with `gemini-3.5-flash` through `env:GEMINI_API_KEY`.
- `qa analyze --upload inline` succeeded on the graded 30-second render with `modelVersion: gemini-3.5-flash`, wrote `reports/gemini-video-qa.json`, and returned video-token usage metadata.
- `color review` succeeded on the graded render with `modelVersion: gemini-3.5-flash`, wrote `reports/color-grade-report.json`, and returned color/exposure notes.
- Root cause for the earlier Files API failure was auth placement on the resumable upload flow. Direct REST probing against Google's documented shell flow showed `media.upload` succeeds when the upload-start URL carries `?key=...`; header-only auth produced misleading `API_KEY_INVALID` failures on the upload/poll lifecycle.
- `vflow` now puts the API key in the Files API upload-start and file-get query strings while keeping `generateContent` on `x-goog-api-key`.
- `qa analyze --upload files` now succeeds on the graded 30-second render with `gemini-3.5-flash`; the uploaded file reached `ACTIVE`, Gemini returned video-token usage metadata, and `reports/gemini-video-qa.json` was written.

## 2026-06-15 Framing Compiler Hardening

Implemented:

- `framing compile` now reads `calibration/framing-presets.json`, `calibration/speaker-map.json`, optional `policy/framing-policy.json`, and `transcript/words.json` as project contracts.
- The compiler writes `decisions/framing-lane.json` and `review/review-queue.json` only with `--commit`; dry-run returns the planned lane and queue without writing.
- Approved presets are validated as source-bounded, stable crop contracts and reject diarization-style `SPEAKER_*` labels in preset IDs or labels.
- Speaker maps are separate from presets and reject unknown preset IDs.
- Frame numbers remain canonical; seconds in framing events are derived from frame/rate values.
- Framing decisions include source media, source word IDs, source frame provenance, preset ID, reason, and review flags.
- Review queue items are generated for unmapped speakers, low-confidence words, overlap/wide fallbacks, and minimum-dwell fallbacks.
- `timeline compile` segments now carry source/timeline frame provenance.
- `render verify` accepts `--project` and resolves project-relative render paths.

Verification:

```bash
go test ./internal/framing -run 'TestCompileLaneUsesSpeakerMapAndQueuesContractExceptions|TestCompileLaneFallsBackToWideForOverlappingSpeakers' -v
go test ./internal/cli -run 'TestRenderVerify(UsesFFProbeJSON|ResolvesProjectRelativeRender)|TestMediaExtractRangesResolvesProjectRelativePaths|TestCutCreateResolvesProjectRelativePaths|TestFramingCompileBuildsLaneAndReviewQueueFromProjectArtifacts' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

Results:

- Targeted framing/CLI tests passed.
- Full tests and vet passed.
- Schema validation passed with command count `60` and schema count `20`.
- Doctor reported `status: ok` with ffmpeg/ffprobe available and OpenAI/Gemini env vars present.
- CLI audit passed at `100/100`.

Still intentionally outside this Go CLI slice:

- Browser calibration UI / React crop editor.
- Full Resolve transform automation.
- Exhaustive real-editor NLE roundtrip fixtures.

## 2026-06-16 Feedback Command Hardening

Implemented:

- `vflow feedback` now records a versioned `vflow-feedback/v1` JSONL entry instead of returning only a placeholder status.
- Feedback writes are dry-run by default and append to `reports/feedback.jsonl` only with `--commit`.
- The command accepts `--project`, `--message`, `--category`, `--source`, and project-relative `--output`.
- Missing feedback messages return structured `vflow-error/v1` JSON with `MISSING_FEEDBACK_MESSAGE`.

Verification:

```bash
go test ./internal/cli
```

## 2026-06-16 NLE Import Path Hardening

Implemented:

- `vflow nle import --project <dir> --input <relative-path>` now resolves timeline input paths relative to the project folder.
- The same project-relative resolver is shared by NLE import, diff, accepted-review loading, and apply paths.
- Added regression coverage for importing `exports/timeline.fcpxml` through `--project`.

Verification:

```bash
go test ./internal/cli -run 'TestNLE' -v
```

## 2026-06-16 Auth Doctor Provider Matrix Hardening

Implemented:

- `vflow auth doctor` now supports `--provider` and `--model`.
- Provider-specific checks cover `openai`, `elevenlabs`, `soniox`, `assemblyai`, `deepgram`, `gladia`, `gemini`, `anthropic`, `huggingface`, and local/import providers.
- Missing keys degrade to structured capability output instead of failing the whole command.
- Live auth/model checks require `--live --commit`; Gemini live checks reuse model listing through the existing QA doctor path.
- Output reports env var names, key presence, capability metadata, default models, and `secrets_redacted: true` without printing secret values.

Verification:

```bash
go test ./internal/cli -run 'Test(AuthDoctor|Profile|Config)' -v
go run ./cmd/vflow auth doctor --provider elevenlabs --format json --format-error json
```

## 2026-06-16 QA Review Queue Hardening

Implemented:

- `vflow qa analyze` now supports `--append-review-queue`.
- Dry-run returns `proposed_review_items` and `review_queue_path` without writing.
- Live Gemini responses can be parsed for high-confidence JSON observations, including JSON embedded in Gemini text parts.
- Only observations with confidence `>= 0.75` become human-review items.
- With `--commit`, proposed items append to `review/review-queue.json` with stable sequential IDs; timeline and framing contracts are not modified.

Verification:

```bash
go test ./internal/cli -run 'TestQA|TestReviewItems|TestAppendReviewQueue' -v
```

## 2026-06-16 Color Render Report Hardening

Implemented:

- `vflow color apply` now supports `--project`, `--intent`, `--qa-report`, and `--ffmpeg-path`.
- Committed LUT renders update `reports/render-report.json` with a separate `color` object.
- The render report records ungraded render path, graded render path, LUT path, LUT SHA-256, ffmpeg filtergraph, color warnings, QA report refs, and preview/final intent.
- Color metadata stays in the render report and does not modify timeline or framing contracts.

Verification:

```bash
go test ./internal/cli -run 'TestColor|TestRender' -v
go test ./internal/color ./internal/render
```

## 2026-06-16 Render Report Schema Hardening

Implemented:

- `schemas/render-report.schema.json` now describes the versioned render report contract instead of accepting any object.
- The schema includes the committed `color` metadata object with required ungraded/graded paths, LUT path, LUT SHA-256, ffmpeg filtergraph, warnings, preview/final intent, and QA report references.
- `vflow color apply` now rejects unsupported `--intent` values before reading LUTs or running ffmpeg, preserving the schema enum.
- Added regression coverage for invalid intent handling and render-report color schema fields.

Verification:

```bash
go test ./internal/cli -run 'TestColor|TestRenderReportSchema|TestSchemaValidate' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 Color Grade Report Schema Hardening

Implemented:

- `schemas/color-grade-report.schema.json` now describes the versioned color review report contract instead of accepting any object.
- The schema requires provider, model, input, report path, confidence, observations, and the optional raw provider response field.
- `vflow color review` now rejects unsupported providers with structured `INVALID_ENUM`.
- Committed non-live color review reports now persist `status: written`, matching the CLI response instead of leaving the file as `planned`.
- Added regression coverage for persisted report status, unsupported provider errors, and color-grade report schema fields.

Verification:

```bash
go test ./internal/cli -run 'TestColor|TestColorGradeReportSchema|TestSchemaValidate' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 Transcript Words Contract Hardening

Implemented:

- `schemas/transcript.schema.json` now describes canonical `transcript/words.json` instead of accepting any object.
- Added transcript validation before importing or writing canonical words.
- Validation now requires `vflow-words/v1`, source media ID, frame rate, stable word IDs/text/provider, non-negative start frames, `end_frame > start_frame`, and confidence values between 0 and 1.
- `vflow transcript import --provider generic-words` now rejects invalid canonical words with structured `TRANSCRIPT_IMPORT_FAILED` before writing `transcript/words.json`.
- Added regression coverage for invalid frame ranges, confidence bounds, CLI rejection, and transcript schema fields.

Verification:

```bash
go test ./internal/transcript ./internal/cli -run 'Test(Import|Validate|Transcript|SchemaValidate|TranscriptSchema)' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 Content EDL and Time Map Contract Hardening

Implemented:

- `schemas/content-edl.schema.json` now describes canonical `decisions/content-edl.json` with version, rate, and half-open delete segments.
- `schemas/time-map.schema.json` now describes generated `decisions/time-map.json` with duration frames and delete ranges.
- Added content EDL validation before importing, reading, or writing cleanup decisions.
- Validation now requires `vflow-content-edl/v1`, frame rate, stable delete IDs, non-negative start frames, `end_frame > start_frame`, confidence values between 0 and 1, and non-overlapping delete ranges.
- `vflow cleanup apply` now reports invalid delete decisions as structured `CONTENT_EDL_INVALID` and does not write `content-edl.json`.
- Added regression coverage for invalid ranges, overlapping deletes, structured CLI errors, and content/time-map schema fields.

Verification:

```bash
go test ./internal/cleanup ./internal/cli -run 'Test(ImportDelete|ValidateContent|Cleanup|Timeline|SchemaValidate|ContentEDL)' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 Source Media Review Contract Hardening

Implemented:

- `schemas/source-media-review.schema.json` now describes canonical `source-media-review.json` with versioned source entries, dimensions, duration, frame rate, timebase, codec, audio streams, VFR/CFR status, warnings, representative frame plan, and optional probe command.
- Added source media review validation for parsed ffprobe data and write paths.
- `vflow media probe --commit` now writes a canonical source media review artifact instead of persisting the CLI response envelope.
- The CLI response still includes status, project, source summaries, and review path for agent workflows.
- Added regression coverage for invalid source dimensions, canonical artifact writes, committed CLI artifact shape, and schema fields.

Verification:

```bash
go test ./internal/media ./internal/cli -run 'Test(ParseFFProbe|ValidateSource|WriteReviews|MediaProbe|SourceMediaReviewSchema|SchemaValidate)' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 Project Contract Hardening

Implemented:

- `schemas/project.schema.json` now describes the complete root `project.json` contract instead of requiring only version and ID.
- Project contracts now require `vflow-project/v1`, a stable ID pattern, root path, `created_at`, and `updated_at`.
- `project init` validates generated project contracts before planning or writing.
- `project get` validates loaded project contracts after root defaulting and rejects malformed IDs or timestamps.
- Project CLI errors for invalid contracts are now structured `PROJECT_INVALID` responses.
- Added regression coverage for invalid IDs, invalid loaded contracts, timestamp ordering, structured CLI errors, and schema fields.

Verification:

```bash
go test ./internal/project -v
go test ./internal/project ./internal/cli -run 'Test(Project|SchemaValidate)' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 Gemini QA Report Contract Hardening

Implemented:

- `schemas/gemini-video-qa.schema.json` now describes the vflow-owned `reports/gemini-video-qa.json` wrapper instead of accepting any object.
- `vflow qa analyze` now includes `version: vflow-gemini-video-qa/v1` in planned/analyzed output.
- Committed live QA reports now persist vflow metadata plus raw Gemini output under `provider_response`, instead of writing raw provider JSON as the whole report.
- The wrapper records provider, model, render path, upload mode, report path, prompt, optional uploaded file metadata, optional review queue refs, and the provider response.
- Added regression coverage for dry-run version output, wrapped provider response writes, and schema fields.

Verification:

```bash
go test ./internal/cli -run 'TestQA|TestWriteGemini|TestGeminiVideoQA|TestSchemaValidate' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 NLE Sidecar Contract Hardening

Implemented:

- Added `schemas/nle-sidecar.schema.json` for the `exports/sidecars/<target>-vflow-sidecar.json` roundtrip contract.
- `vflow schema --validate` now includes the NLE sidecar schema in artifact validation.
- `nle export` now rejects unsupported or mistyped targets instead of silently emitting a generic sidecar for a bad target name.
- Added regression coverage for valid NLE targets, unsupported target errors, schema inventory, and required sidecar segment mapping fields.

Verification:

```bash
go test ./internal/nle ./internal/cli -run 'Test(ValidTarget|Export|NLE|Schema|GeminiVideoQA|NLESidecar)' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 NLE Missing Sidecar Guardrail

Implemented:

- `nle diff` now blocks identity-sensitive NLE changes that arrive without a vflow segment ID instead of allowing them into `safe_merge`.
- Raw editor EDL imports without `* VFLOW-SEGMENT-ID` now classify as `missing_sidecar` in the blocked bucket.
- Added package and CLI regression coverage so missing sidecar identity is enforced both in `internal/nle` and through `vflow nle diff`.

Verification:

```bash
go test ./internal/nle ./internal/cli -run 'Test(NLE|ParseEDL|ClassifyBlocks|ApplyPlan|AcceptedReview)' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 NLE Marker Identity Hardening

Implemented:

- FCPXML/Premiere XML marker `value` fields are no longer treated as vflow segment IDs unless they explicitly contain `vflow:segment-id=...`.
- Plain producer/editor marker text without vflow identity now falls through to the missing-sidecar guardrail and is blocked from safe merge.
- Added regression coverage for both plain marker values and explicit vflow marker values.

Verification:

```bash
go test ./internal/nle -run 'Test(FCPXMLMarker|ParseImport|ParseEDL|ClassifyBlocks)' -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 Doctor NLE Capability Evidence

Implemented:

- `vflow doctor --local --format json` now reports NLE targets, import formats, sidecar export support, blocked roundtrip change types, Resolve package handling, and the remaining real-editor fixture proof gap.
- CI now runs `doctor --local` to match the plan's local proof gate.
- `Makefile` now includes `doctor-local` for explicit local-only checks.
- Added regression coverage for NLE capability fields in doctor output.

Verification:

```bash
go test ./internal/cli -run 'TestDoctorReportsNLECapabilities|TestAuthDoctor|TestSchemaValidateReportsCoverage' -v
go run ./cmd/vflow doctor --local --format json --format-error json
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 NLE Retime And Media Replacement Import Hardening

Implemented:

- FCPXML `timeMap`/`timept` imports now classify as `speed_change` in the needs-review bucket.
- FCPXML `media-rep` imports now classify as `media_replace` in the needs-review bucket.
- OTIO time-warp style effects now classify as `speed_change` without replacing the current clip segment identity with the effect name.

Verification:

```bash
go test ./internal/nle -run 'TestParse(FCPXML|OTIO)|TestClassifyRoutesKnown|TestAcceptedReview' -v
```

## 2026-06-16 Gemini Files API Upload Rejection Article

Implemented:

- Added `docs/research/gemini-files-api-upload-rejection-notes.md` as a publishable debugging article.
- Documented the working Files API mental model, REST upload sequence, auth-placement mismatch, MIME pitfalls, file-state polling, `file_data` payload shape, model drift, and the `vflow` live proof path.
- Kept the article free of raw secrets and private media details.

## 2026-06-16 Provider Bakeoff Schema Contract

Implemented:

- Added `schemas/provider-bakeoff.schema.json` for `reports/provider-bakeoff.json`.
- `transcript bakeoff` now writes `version: vflow-provider-bakeoff/v1` into the committed report envelope.
- `vflow schema --validate` now includes the provider bakeoff report schema in the artifact inventory.

Verification:

```bash
go test ./internal/cli -run 'Test(SchemaValidateReportsCoverage|ProviderBakeoffSchema|TranscriptBakeoff)' -v
```

## 2026-06-16 Audit Report Schema Contract

Implemented:

- Added `schemas/audit-report.schema.json` for the `vflow-cli-audit/v1` scorecard contract.
- `vflow schema --validate` now includes the audit report schema in the artifact inventory.
- Added regression coverage for audit version, pass/fail status, and required summary fields.

Verification:

```bash
go test ./internal/cli -run 'Test(SchemaValidateReportsCoverage|AuditReportSchema)' -v
go run ./cmd/vflow audit cli --format json --format-error json
```

## 2026-06-16 Install And Self-Update Hardening

Implemented:

- Replaced the placeholder installer with `scripts/install.sh`, which downloads the matching release archive, verifies `checksums.txt`, installs to `${VFLOW_BIN_DIR:-$HOME/.local/bin}`, and backs up any existing binary.
- `vflow upgrade --commit --install-dir <dir>` now downloads the latest matching release archive, verifies SHA256 from release checksums, extracts the binary, backs up an existing installation, and atomically installs the replacement.
- Added regression coverage for checksum-verified staging, checksum mismatch rejection, and install backup behavior.

Verification:

```bash
go test ./internal/update -v
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
make doctor-local
VFLOW_VERSION=v0.1.4 VFLOW_BIN_DIR="$(mktemp -d)" scripts/install.sh
```
