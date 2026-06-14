# vflow Implementation Report

Date: 2026-06-14

## Implemented

- Go/Cobra CLI at `cmd/vflow/main.go`.
- Structured JSON response and error envelopes.
- Command registry and `vflow schema --validate`.
- `agent-context`, `skill-path`, `doctor`, `audit cli`, config/profile/auth inspection.
- Project init/get and canonical folder layout.
- Media ingest/probe with live `ffprobe` support and copied 4K source discovery.
- Transcript import for `generic-words` and `plain-text`.
- Cleanup delete-segment import and frame-based content EDL.
- Source-frame anchored time map and compiled timeline.
- Framing preset import/list/validate with diarization-label rejection.
- Live ffmpeg preview render and render report.
- Gemini QA hooks with live inline-video call path.
- `.cube` LUT validation and live ffmpeg `lut3d` apply path.
- NLE export sidecar support plus FCPXML/EDL/MLT/OTIO text exporters.
- CI, GoReleaser config, install script, repo `AGENTS.md`, root `SKILL.md`, bundled workflow skill, schemas, and docs research notes.

## Verification Commands

```bash
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json
go run ./cmd/vflow doctor --local --format json
go run ./cmd/vflow audit cli --format json
go build -o bin/vflow ./cmd/vflow
./bin/vflow transcript create --provider nope --format json
```

Proof files:

- `/tmp/vflow-schema.json`
- `/tmp/vflow-doctor.json`
- `/tmp/vflow-audit.json`
- `/tmp/vflow-exit.err`

## Synthetic Live Slice

Synthetic source:

- `fixtures/media/tiny/source.mp4`

Commands were run with real writes and ffmpeg renders into `/tmp/vflow-demo`.

Proof files:

- `/tmp/vflow-proof-01-project.json`
- `/tmp/vflow-proof-02-ingest.json`
- `/tmp/vflow-proof-03-probe.json`
- `/tmp/vflow-proof-04-transcript.json`
- `/tmp/vflow-proof-05-cleanup.json`
- `/tmp/vflow-proof-06-framing.json`
- `/tmp/vflow-proof-07-timeline.json`
- `/tmp/vflow-proof-08-render.json`
- `/tmp/vflow-proof-09-color.json`
- `/tmp/vflow-proof-10-nle.json`

Created artifacts:

- `/tmp/vflow-demo/renders/rough-preview.mp4`
- `/tmp/vflow-demo/renders/rough-preview-graded.mp4`
- `/tmp/vflow-demo/exports/timeline.fcpxml`
- `/tmp/vflow-demo/exports/sidecars/fcpxml-vflow-sidecar.json`

## Real CAIR-GA Copied Fixture

Fixture:

```text
work/test-projects/cair-ga-10yr-executive-directors-30s-highlight
```

Commands ran against copied files under `media/source-4k`. No `/Volumes/Shams Drive` path was used.

Probe proof: `/tmp/vflow-real-02-probe.json`

Confirmed sources:

- `media/source-4k/Executive Directors 12mm 4K 02.MP4` - 3840x2160 H.264
- `media/source-4k/Executive Directors 12mm 4K 03.MP4` - 3840x2160 H.264
- `media/source-4k/Executive Directors 7mm 4K 02.MP4` - 3840x2160 H.264
- `media/source-4k/Executive Directors 7mm 4K 03.MP4` - 3840x2160 H.264

Created real fixture artifacts:

- `source-media-review.json`
- `transcript/words.json`
- `timeline/compiled-timeline.json`
- `renders/rough-preview.mp4`
- `renders/rough-preview-graded.mp4`
- `exports/timeline.fcpxml`
- `exports/sidecars/fcpxml-vflow-sidecar.json`

Proof files:

- `/tmp/vflow-real-01-project.json`
- `/tmp/vflow-real-02-probe.json`
- `/tmp/vflow-real-03-transcript.json`
- `/tmp/vflow-real-04-timeline.json`
- `/tmp/vflow-real-05-render.json`
- `/tmp/vflow-real-06-color.json`
- `/tmp/vflow-real-07-nle.json`
- `/tmp/vflow-real-08-qa.err`

## Live Provider Results

- OpenAI runtime key: live `/v1/models` call succeeded with 122 visible models.
- Gemini runtime key: live models and generateContent calls returned `400 Bad Request`; the CLI surfaced this as structured `GEMINI_QA_FAILED`.
- ElevenLabs, Soniox, AssemblyAI, Deepgram, Gladia, Anthropic, and Hugging Face were not present in runtime env or Secret Gate. Chat-pasted keys were treated as exposed and were not written or used.

## Remaining Work

- Replace minimal NLE text exporters with richer format-specific writers.
- Add real OpenAI/ElevenLabs/Soniox/AssemblyAI/Deepgram/Gladia STT adapters once secrets are available through runtime env or Secret Gate.
- Expand Gemini Files API upload path for large videos; current live path uses inline video.
- Raise `audit cli` threshold from 65 to 80 before alpha.
