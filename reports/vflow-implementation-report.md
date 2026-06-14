# vflow Implementation Report

Date: 2026-06-14

## Implemented

- Go/Cobra CLI with structured `vflow-response/v1` and `vflow-error/v1` envelopes.
- Schema and agent introspection: `schema --validate`, `agent-context`, `skill-path`, `doctor`, `audit cli`.
- Commit-gated project/media/transcript/cleanup/framing/timeline/render/QA/color/NLE/artifact commands.
- YAML-backed `config` and `profile` commands using `VFLOW_CONFIG_PATH` for isolated tests and `~/.vflow/config.yaml` by default.
- Durable JSON job ledger under `project/jobs/` with `jobs list/get/resume`; committed preview renders now write job records.
- Atomic file artifact delivery with overwrite gating.
- Live `ffprobe` source review, ffmpeg preview renders, ffmpeg LUT renders, render verification, and NLE sidecars.
- Render verification parses ffprobe JSON/evidence for duration, resolution, codec, audio streams, and frame count.
- Live OpenAI STT adapter using `OPENAI_API_KEY` and `/v1/audio/transcriptions`; secrets are env-only.
- Gemini QA/color hooks using `GEMINI_API_KEY`, `GOOGLE_API_KEY`, or `GOOGLE_GENERATIVE_AI_API_KEY` with `x-goog-api-key`; provider errors are compact and redacted.
- NLE export/import/diff/apply surfaces for FCPXML, EDL, OTIO, MLT, Resolve alias, Premiere alias, and sidecars.
- Cleanup review and NLE diff can deliver HTML review artifacts.
- Color review writes `reports/color-grade-report.json` without requiring live Gemini, and live Gemini can enrich it when credentials work.
- Public-repo support files: `AGENTS.md`, root `SKILL.md`, bundled workflow skill, schemas, CI, GoReleaser config, install script, and research notes.

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
- `schema --validate` returned `status: valid` and `command_count: 53`.
- `doctor` found `ffmpeg`, `ffprobe`, and `python3`; `OPENAI_API_KEY` and `GEMINI_API_KEY` were present, all other optional provider env vars were absent.
- `audit cli` returned score `72` with pass threshold `65`.

Continuation verification also passed:

```bash
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json
go run ./cmd/vflow audit cli --format json
go run ./cmd/vflow doctor --format json
```

Additional proof commands:

```bash
VFLOW_CONFIG_PATH=tmp/continuation-proof/config.yaml go run ./cmd/vflow profile set --name cont --provider elevenlabs --api-key-env ELEVENLABS_API_KEY --commit --format json
VFLOW_CONFIG_PATH=tmp/continuation-proof/config.yaml go run ./cmd/vflow config set-defaults --project-root ./work --commit --format json
go run ./cmd/vflow render verify --render rough-preview.mp4 --ffprobe-json fixtures/media/tiny/ffprobe.json --expected-width 1920 --expected-height 1080 --expected-duration 12.345 --format json
go run ./cmd/vflow artifacts deliver --input tmp/continuation-proof/project/reports-source.json --deliver file:tmp/continuation-proof/project/reports-copy.json --commit --overwrite --format json
go run ./cmd/vflow cleanup review --project tmp/continuation-proof/project --deliver file:tmp/continuation-proof/project/review/cleanup-review.html --commit --format json
go run ./cmd/vflow nle import --project tmp/continuation-proof/project --input tmp/continuation-proof/project/timeline.fcpxml --commit --format json
go run ./cmd/vflow nle diff --project tmp/continuation-proof/project --import tmp/continuation-proof/project/imports/nle-import.json --deliver file:tmp/continuation-proof/project/review/roundtrip-review.html --format json
go run ./cmd/vflow color review --project tmp/continuation-proof/project --input tmp/continuation-proof/project/renders/rough-preview.mp4 --commit --format json
```

Results:

- Config/profile writes persisted and `config inspect` stayed redacted.
- Render verification returned `status: valid`, `width: 1920`, `height: 1080`, `audio_streams: 1`, and `frame_count: 370`.
- Artifact delivery wrote with `status: delivered`.
- Cleanup review wrote `review/cleanup-review.html`.
- NLE import wrote `imports/nle-import.json`; NLE diff wrote `review/roundtrip-review.html`.
- Color review wrote `reports/color-grade-report.json`.

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
go run ./cmd/vflow nle apply --input tmp/live-demo/exports/sidecars/fcpxml-vflow-sidecar.json --commit --format json
```

NLE proof result: all seven exports wrote sidecars with two compiled segments.

## Live Gemini Result

Live Gemini calls were attempted with the runtime env key:

```bash
go run ./cmd/vflow qa doctor --provider gemini --live --commit --format json --format-error json
go run ./cmd/vflow qa analyze --project tmp/live-demo --render tmp/live-demo/renders/rough-preview.mp4 --provider gemini --live --commit --format json --format-error json
go run ./cmd/vflow color review --input tmp/live-demo/renders/rough-preview-graded.mp4 --provider gemini --live --commit --format json --format-error json
```

Provider result: Google returned `400 Bad Request` with `API key expired. Please renew the API key.` The CLI surfaced this as structured `QA_DOCTOR_FAILED`, `GEMINI_QA_FAILED`, and `COLOR_REVIEW_FAILED` errors without printing the raw key.

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
go run ./cmd/vflow nle diff --import work/test-projects/cair-ga-10yr-executive-directors-30s-highlight/exports/timeline.fcpxml --format json
```

Fixture proof:

- `media probe` wrote four copied sources, each 3840x2160 H.264 at `24000/1001`.
- Reference transcript import wrote 19,273 canonical words.
- Timeline compile wrote one kept segment for 2,220 frames.
- Preview render verified as H.264, 1920x1080, 1.001000 seconds.
- LUT render wrote `renders/rough-preview-graded.mp4`.
- FCPXML export wrote one-segment sidecar.

## Remaining Work

- Replace minimal NLE text bodies with richer format-specific timeline XML/OTIO/EDL/MLT writers and stronger import parsing.
- Add live adapters for ElevenLabs, Soniox, AssemblyAI, Deepgram, and Gladia once runtime keys are available.
- Add Gemini Files API upload path for large videos after rotating the expired key.
- Add SQLite/FTS project indexing if the plan remains strict on that storage backend.
- Raise `audit cli` target from 72 toward the planned 80+ alpha threshold.
