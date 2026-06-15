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
- `schema --validate` returned `status: valid` and `command_count: 55`.
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
- `schema --validate` returned `status: valid` and `command_count: 55`.
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
VFLOW_INDEX_PATH=tmp/index-proof/index.sqlite go run ./cmd/vflow project index --path work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --commit --format json
VFLOW_INDEX_PATH=tmp/index-proof/index.sqlite go run ./cmd/vflow transcript search --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --query Executive --data-source local --limit 5 --format json
```

Results:

- Config/profile writes persisted and `config inspect` stayed redacted.
- Render verification returned `status: valid`, `width: 1920`, `height: 1080`, `audio_streams: 1`, and `frame_count: 370`.
- Artifact delivery wrote with `status: delivered`.
- Cleanup review wrote `review/cleanup-review.html`.
- NLE import wrote `imports/nle-import.json`; NLE diff wrote `review/roundtrip-review.html`.
- Fixture NLE diff classified `clip_trim`, `marker_note`, and `audio_level` as safe, `crop_change` and `title_card` as needs-review, `color_grade` as blocked, and `unclassified: []`.
- Color review wrote `reports/color-grade-report.json`.
- SQLite project index wrote `tmp/index-proof/index.sqlite` and fixture `reports/provenance.json`; local FTS transcript search returned five `Executive` hits with project ID, word IDs, and frame ranges.

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

## Real CAIR-GA Copied Fixture

Fixture:

```text
work/test-projects/cair-ga-10yr-executive-directors-30s-highlight
```

Commands ran only against copied files under `media/source-4k`. No `/Volumes/Shams Drive` path was used.

```bash
go run ./cmd/vflow project get --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --format json
go run ./cmd/vflow media probe --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --commit --format json
go run ./cmd/vflow transcript import --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --provider plain-text --input work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/transcript/Executive-Directors.named.txt --rate 24000/1001 --commit --format json
go run ./cmd/vflow transcript align --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --commit --format json
go run ./cmd/vflow timeline compile --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --duration-frames 2220 --commit --format json
go run ./cmd/vflow render preview --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --source "work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/media/source-4k/Executive Directors 12mm 4K 02.MP4" --duration-seconds 1 --commit --format json
go run ./cmd/vflow render verify --input work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/renders/rough-preview.mp4 --format json
go run ./cmd/vflow color apply --input work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/renders/rough-preview.mp4 --lut fixtures/color/basic.cube --deliver file:work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/renders/rough-preview-graded.mp4 --commit --format json
go run ./cmd/vflow nle export --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --target fcpxml --deliver file:work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/exports/timeline.fcpxml --commit --format json
go run ./cmd/vflow nle import --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --input work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/exports/timeline.fcpxml --commit --format json
go run ./cmd/vflow nle diff --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --import work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/imports/nle-import.json --deliver file:work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/review/roundtrip-review.html --format json
go run ./cmd/vflow nle apply --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --input work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/imports/nle-import.json --commit --format json --format-error json
VFLOW_INDEX_PATH=tmp/index-proof/index.sqlite go run ./cmd/vflow project index --path work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --commit --format json
VFLOW_INDEX_PATH=tmp/index-proof/index.sqlite go run ./cmd/vflow transcript search --project work/test-projects/cair-ga-10yr-executive-directors-30s-highlight --query Executive --data-source local --limit 5 --format json
```

Fixture proof:

- `media probe` wrote four copied sources, each 3840x2160 H.264 at `24000/1001`.
- Reference transcript import wrote 19,273 canonical words.
- Timeline compile wrote one kept segment for 2,220 frames.
- Preview render verified as H.264, 1920x1080, 1.001000 seconds.
- Actual 30-second CLI cut wrote `renders/cair-ga-actual-30s.mp4` from `Executive Directors 12mm 4K 02.MP4` using `render preview --start-seconds 10 --duration-seconds 30 --output renders/cair-ga-actual-30s.mp4 --commit`.
- The 30-second cut verified as H.264/AAC, 1920x1080, 30.03 seconds, with one audio stream.
- Transcript-selected multi-segment cut wrote `renders/cair-ga-transcript-social-30s-v2.mp4` from `decisions/social-30s-transcript-cut-v2.json` using `render transcript-cut --commit`.
- The transcript cut uses three transcript ranges, alternates `7mm` wide, `12mm` tighter, then `7mm` wide again, and verified as H.264/AAC, 1920x1080, 30.03 seconds, with no source timecode/data stream.
- LUT render wrote `renders/rough-preview-graded.mp4`.
- FCPXML export wrote one-segment sidecar.
- FCPXML import wrote `imports/nle-import.json` with `clip_trim` and `marker_note`.
- NLE diff wrote `review/roundtrip-review.html` with two `safe_merge` changes and no needs-review, blocked, or unclassified changes.
- Guarded NLE apply wrote `imports/applied-nle-changes.json`.
- Project index wrote SQLite/FTS rows for one project, four sources, 19,273 transcript words, 15 artifacts, and four NLE events.
- Local FTS search returned five `Executive` transcript matches from the fixture with canonical frame ranges.

Latest render note: the 30-second CAIR-GA clips were rendered only from copied `media/source-4k` files inside `work/test-projects`; no `/Volumes/Shams Drive` path was used.

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

Group 4 proof target:

```text
work/test-projects/cair-ga-group-4-current-board-social-30s
```

Read-only source inputs:

```text
/Volumes/Shams Drive/CAIR-GA 10 yr/Group 4 Current Board/Camera Source Files/Group 4 12mm 4K 01.MP4
/Volumes/Shams Drive/CAIR-GA 10 yr/Group 4 Current Board/Camera Source Files/Group 4 9mm 4K 01.MP4
/Volumes/Shams Drive/CAIR-GA 10 yr/Group 4 Current Board/Camera Source Files/Group 4 7mm 4K 01.MP4
```

Proof artifacts:

- `work/test-projects/cair-ga-group-4-current-board-social-30s/calibration/media-sync-map.json`
- `work/test-projects/cair-ga-group-4-current-board-social-30s/calibration/source-range-manifest.json`
- `work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-sync-proof-cut.json`
- `work/test-projects/cair-ga-group-4-current-board-social-30s/media/sync-ranges/group4_known_cta_30s-group4_7mm.mp4`
- `work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-proof-30s.mp4`
- `work/test-projects/cair-ga-group-4-current-board-social-30s/reports/group4-sync-proof-transcript-proof.json`
- `work/test-projects/cair-ga-group-4-current-board-social-30s/live-transcript-proof/transcript/openai-transcription.json`

Alignment check:

- Transcript `34:19` is `2059` seconds.
- 12mm source start resolved to `2415` seconds (`40:15`).
- 9mm source start resolved to `2415` seconds (`40:15`).
- 7mm source start resolved to `2432` seconds (`40:32`).

Commands:

```bash
go run ./cmd/vflow media extract-ranges --project work/test-projects/cair-ga-group-4-current-board-social-30s --sync-map work/test-projects/cair-ga-group-4-current-board-social-30s/calibration/media-sync-map.json --ranges work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-known-alignment-check-ranges.json --output-dir work/test-projects/cair-ga-group-4-current-board-social-30s/media/sync-ranges --format json --format-error json
go run ./cmd/vflow media extract-ranges --project work/test-projects/cair-ga-group-4-current-board-social-30s --sync-map work/test-projects/cair-ga-group-4-current-board-social-30s/calibration/media-sync-map.json --ranges work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-sync-proof-ranges.json --output-dir work/test-projects/cair-ga-group-4-current-board-social-30s/media/sync-ranges --commit --timeout 10m --format json --format-error json
go run ./cmd/vflow cut create --project work/test-projects/cair-ga-group-4-current-board-social-30s --ranges work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-sync-proof-ranges.json --sync-map work/test-projects/cair-ga-group-4-current-board-social-30s/calibration/media-sync-map.json --output work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-sync-proof-cut.json --commit --format json --format-error json
go run ./cmd/vflow render transcript-cut --project work/test-projects/cair-ga-group-4-current-board-social-30s --input work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-sync-proof-cut.json --sync-map work/test-projects/cair-ga-group-4-current-board-social-30s/calibration/media-sync-map.json --output renders/group4-sync-proof-30s.mp4 --commit --timeout 10m --format json --format-error json
go run ./cmd/vflow render verify --render work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-proof-30s.mp4 --expected-duration 30 --expected-width 1920 --expected-height 1080 --format json --format-error json
go run ./cmd/vflow render verify-transcript --project work/test-projects/cair-ga-group-4-current-board-social-30s --render work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-proof-30s.mp4 --cut work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-sync-proof-cut.json --output work/test-projects/cair-ga-group-4-current-board-social-30s/reports/group4-sync-proof-transcript-proof.json --commit --format json --format-error json
go run ./cmd/vflow transcript create --project work/test-projects/cair-ga-group-4-current-board-social-30s/live-transcript-proof --provider openai --source work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-proof-30s.mp4 --live --commit --timeout 5m --format json --format-error json
```

Render proof:

- `render verify` returned `status: valid`.
- Render path: `work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-proof-30s.mp4`.
- Dimensions: `1920x1080`.
- Duration: `30.03` seconds.
- Codec/audio: H.264 with one audio stream.
- OpenAI live STT proof wrote 82 words to the isolated `live-transcript-proof` folder.

## 2026-06-15 Multi-Angle Sync Cut And Grade

Commands:

```bash
go run ./cmd/vflow media extract-ranges --project work/test-projects/cair-ga-group-4-current-board-social-30s --sync-map calibration/media-sync-map.json --ranges decisions/group4-sync-multiangle-ranges.json --output-dir media/sync-ranges-multiangle --manifest calibration/source-range-manifest-multiangle.json --commit --timeout 20m --format json --format-error json
go run ./cmd/vflow cut create --project work/test-projects/cair-ga-group-4-current-board-social-30s --sync-map calibration/media-sync-map.json --ranges decisions/group4-sync-multiangle-ranges.json --output decisions/group4-sync-multiangle-cut.json --commit --format json --format-error json
go run ./cmd/vflow render transcript-cut --project work/test-projects/cair-ga-group-4-current-board-social-30s --input work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-sync-multiangle-cut.json --sync-map work/test-projects/cair-ga-group-4-current-board-social-30s/calibration/media-sync-map.json --output renders/group4-sync-multiangle-social-30s.mp4 --commit --timeout 30m --format json --format-error json
go run ./cmd/vflow render verify --render work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-multiangle-social-30s.mp4 --expected-width 1920 --expected-height 1080 --expected-duration 30.03 --format json --format-error json
go run ./cmd/vflow color apply --input work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-multiangle-social-30s.mp4 --lut work/test-projects/cair-ga-group-4-current-board-social-30s/calibration/group4-natural-contrast-rfast.cube --deliver file:work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-multiangle-social-30s-graded-natural-v2.mp4 --commit --fields status,plan --format json --format-error json
go run ./cmd/vflow render verify --render work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-multiangle-social-30s-graded-natural-v2.mp4 --expected-width 1920 --expected-height 1080 --expected-duration 30.03 --format json --format-error json
go run ./cmd/vflow render verify-transcript --project work/test-projects/cair-ga-group-4-current-board-social-30s --render work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-multiangle-social-30s.mp4 --cut work/test-projects/cair-ga-group-4-current-board-social-30s/decisions/group4-sync-multiangle-cut.json --output work/test-projects/cair-ga-group-4-current-board-social-30s/reports/group4-sync-multiangle-transcript-proof.json --commit --format json --format-error json
go run ./cmd/vflow transcript create --provider openai --source work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-multiangle-social-30s.mp4 --project work/test-projects/cair-ga-group-4-current-board-social-30s/live-transcript-proof-sync-multiangle --live --commit --timeout 5m --format json --format-error json
go run ./cmd/vflow color review --project work/test-projects/cair-ga-group-4-current-board-social-30s --input work/test-projects/cair-ga-group-4-current-board-social-30s/renders/group4-sync-multiangle-social-30s-graded-natural-v2.mp4 --provider gemini --live --commit --timeout 3m --format json --format-error json
```

Proof:

- `group4-sync-multiangle-social-30s.mp4`: valid H.264/AAC, `1920x1080`, `30.03s`, one audio stream, 720 frames.
- `group4-sync-multiangle-social-30s-graded-natural-v2.mp4`: valid H.264/AAC, `1920x1080`, `30.03s`, one audio stream, 720 frames.
- OpenAI STT returned 73 words and matched the planned summary: CAIR as first line of defense, shield for the community, and an organization that has your back / will fight for you / protect you / empower that organization.
- Gemini color review attempted live but returned `API key expired` with `API_KEY_INVALID`; rotate the key before using Gemini QA again.

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
