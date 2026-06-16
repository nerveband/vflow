# vflow

`vflow` is an agent-native Go CLI for local-first video editing workflows. It turns a project folder into inspectable JSON contracts, deterministic preview renders, provider QA reports, color/LUT artifacts, and portable NLE exchange files with guarded roundtrip import.

The design goal is simple: agents can suggest edits, but `vflow` owns validation, canonical artifacts, frame math, safety gates, and writes.

## Status

Current alpha capabilities:

- Structured JSON output and structured JSON errors for agent workflows.
- Command/schema introspection through `schema`, `agent-context`, `skill-path`, `doctor`, and `audit cli`.
- Dry-run by default for mutating work, with `--commit` required for writes and `--live` required for provider calls.
- Local project/media/transcript/cleanup/framing/timeline/render/color/NLE workflows.
- ffprobe and ffmpeg adapters for media inspection, samples, proxy/range extraction, preview rendering, verification, transcript-cut rendering, and LUT application.
- Live STT adapters for OpenAI, ElevenLabs, Deepgram, AssemblyAI, Gladia, and Soniox.
- Gemini video QA through inline and Files API upload modes, including uploaded-file polling until `ACTIVE`.
- Color review and LUT workflows with Gemini enrichment when credentials are present.
- NLE export/import/diff/apply/accept support for Resolve-style FCPXML, FCPXML, EDL, OTIO, MLT, and Premiere XMEML where practical.
- Versioned artifact schemas for command output, project contracts, transcripts, timelines, QA reports, provider bakeoffs, NLE diffs/sidecars, render reports, and audit reports.

See [reports/vflow-completion-audit.md](reports/vflow-completion-audit.md) for current proof and known gaps. The main remaining compatibility gap is exhaustive proof against real exported timelines from every target editor.

## Install And Build

Prerequisites:

- Go 1.25.x or newer compatible local Go toolchain.
- `ffmpeg` and `ffprobe` on `PATH` for media/render workflows.
- Optional provider API keys in runtime environment variables for live STT/Gemini workflows.

Build:

```bash
go build -o bin/vflow ./cmd/vflow
bin/vflow version --format json
```

Install from GitHub Releases:

```bash
curl -fsSL https://raw.githubusercontent.com/nerveband/vflow/main/scripts/install.sh | sh
```

The installer downloads the latest release archive, verifies `checksums.txt`, installs `vflow` into `${VFLOW_BIN_DIR:-$HOME/.local/bin}`, and backs up an existing binary as `vflow.bak`.

Common development gates:

```bash
go test ./...
go vet ./...
go run ./cmd/vflow schema --validate --format json --format-error json
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow audit cli --format json --format-error json
make doctor-local
```

The same commands are the baseline proof set for changes to the CLI.

## Safety Model

`vflow` is built for automation, but it is deliberately conservative:

- Mutating commands support `--dry-run`.
- Writes require `--commit`.
- Live provider calls require `--live`; costly or writing live calls should also use `--commit`.
- Provider secrets are read from runtime environment variables or external secret tooling, not from project files.
- `work/`, `tmp/`, `bin/`, `dist/`, `.env`, and private copied media are ignored and should not be published.
- NLE files are adapters. Canonical project state remains in JSON artifacts owned by `vflow`.

Use JSON in agent workflows:

```bash
go run ./cmd/vflow doctor --format json --format-error json
go run ./cmd/vflow schema --validate --format json --format-error json
```

Error responses use the versioned `vflow-error/v1` shape with `code`, `message`, `hint`, `retryable`, and `exit_code`.

## Project Folder Contract

A typical project folder contains:

```text
project.json
source-media-review.json
media/
transcript/
calibration/
policy/
decisions/
timeline/
review/
renders/
exports/
imports/
reports/
jobs/
```

Canonical artifacts include:

- `project.json`
- `source-media-review.json`
- `transcript/words.json`
- `decisions/content-edl.json`
- `decisions/time-map.json`
- `calibration/framing-presets.json`
- `calibration/speaker-map.json`
- `decisions/framing-lane.json`
- `timeline/compiled-timeline.json`
- `review/review-queue.json`
- `reports/render-report.json`
- `reports/gemini-video-qa.json`
- `reports/color-grade-report.json`
- `reports/provider-bakeoff.json`
- `reports/provenance.json`

Schemas live in [schemas](schemas). Check them with:

```bash
go run ./cmd/vflow schema --validate --format json --format-error json
```

## Core Commands

System and introspection:

```bash
vflow version
vflow schema --validate
vflow agent-context
vflow skill-path
vflow doctor
vflow audit cli
vflow feedback
```

Project and config:

```bash
vflow project init
vflow project get
vflow project list
vflow project index
vflow config inspect
vflow config defaults
vflow profile set
vflow profile use
vflow auth doctor
```

Media, transcript, timeline, and render:

```bash
vflow media probe
vflow media ingest
vflow media proxy
vflow media samples
vflow media sync
vflow media extract-ranges
vflow transcript import
vflow transcript create
vflow transcript bakeoff
vflow transcript search
vflow cleanup review
vflow cleanup apply
vflow framing calibrate
vflow framing crop
vflow framing reframe
vflow framing zoom
vflow framing compile
vflow timeline compile
vflow render preview
vflow render transcript-cut
vflow render verify
vflow render verify-transcript
```

QA, color, and NLE:

```bash
vflow qa doctor
vflow qa analyze
vflow color review
vflow color apply
vflow color export-lut
vflow nle export
vflow nle import
vflow nle diff
vflow nle accept
vflow nle apply
```

## Local-First Workflow

Initialize a project:

```bash
go run ./cmd/vflow project init \
  --path tmp/demo \
  --id demo \
  --commit \
  --format json \
  --format-error json
```

Probe media:

```bash
go run ./cmd/vflow media probe \
  --project tmp/demo \
  --source tmp/demo/media/source.mp4 \
  --commit \
  --format json \
  --format-error json
```

Import a transcript:

```bash
go run ./cmd/vflow transcript import \
  --project tmp/demo \
  --provider plain-text \
  --input transcript.txt \
  --commit \
  --format json \
  --format-error json
```

Calibrate approved framing presets:

```bash
go run ./cmd/vflow framing calibrate \
  --project tmp/demo \
  --source media/source.mp4 \
  --listen 127.0.0.1:0 \
  --open=false \
  --session-timeout 30m \
  --commit \
  --format json \
  --format-error json
```

The command serves an embedded local UI on `127.0.0.1`, returns JSON with `session_id`, `url`, health/status/shutdown URLs, artifact paths, port, PID, timeout, and shutdown-token presence, then waits until shutdown or timeout. `framing crop`, `framing zoom`, `framing reframe`, `framing frame`, `framing crop-calibrate`, `framing zoom-calibrate`, and `framing preset-calibrate` are aliases for the same session because calibration means approving the crop rectangles that later produce zoomed reframes. Agents can use `--wait=false` to return after session metadata is written under `tmp/sessions/`, or keep the process running and call `GET /api/state`, `POST /api/presets`, `POST /api/speaker-map`, `POST /api/policy`, `POST /api/commit?commit=true`, and `POST /api/shutdown`.

Compile timeline artifacts:

```bash
go run ./cmd/vflow timeline compile \
  --project tmp/demo \
  --commit \
  --format json \
  --format-error json
```

Render and verify:

```bash
go run ./cmd/vflow render preview \
  --project tmp/demo \
  --commit \
  --format json \
  --format-error json

go run ./cmd/vflow render verify \
  --project tmp/demo \
  --render renders/rough-preview.mp4 \
  --format json \
  --format-error json
```

## Live Providers

Provider keys are optional. Local workflows should work without API keys.

Supported environment variables include:

```text
OPENAI_API_KEY
ELEVENLABS_API_KEY
DEEPGRAM_API_KEY
ASSEMBLYAI_API_KEY
GLADIA_API_KEY
SONIOX_API_KEY
GEMINI_API_KEY
GOOGLE_API_KEY
GOOGLE_GENERATIVE_AI_API_KEY
ANTHROPIC_API_KEY
HF_TOKEN
HUGGINGFACE_TOKEN
```

Check redacted provider readiness:

```bash
go run ./cmd/vflow auth doctor --format json --format-error json
go run ./cmd/vflow qa doctor --provider gemini --model gemini-3.5-flash --live --format json --format-error json
```

Run a live STT bakeoff only when you intend to spend provider quota:

```bash
go run ./cmd/vflow transcript bakeoff \
  --project tmp/demo \
  --source tmp/demo/media/source.mp4 \
  --providers openai,elevenlabs,soniox,assemblyai,deepgram,gladia,local \
  --live \
  --commit \
  --timeout 20m \
  --format json \
  --format-error json
```

Run Gemini QA:

```bash
go run ./cmd/vflow qa analyze \
  --project tmp/demo \
  --render tmp/demo/renders/rough-preview.mp4 \
  --provider gemini \
  --model gemini-3.5-flash \
  --upload files \
  --live \
  --commit \
  --timeout 5m \
  --format json \
  --format-error json
```

Gemini Files API implementation notes are documented in [docs/research/gemini-files-api-upload-rejection-notes.md](docs/research/gemini-files-api-upload-rejection-notes.md).

## NLE Exchange And Roundtrip

Export:

```bash
go run ./cmd/vflow nle export \
  --project tmp/demo \
  --target fcpxml \
  --deliver file:tmp/demo/exports/timeline.fcpxml \
  --commit \
  --format json \
  --format-error json
```

Import an edited interchange file:

```bash
go run ./cmd/vflow nle import \
  --project tmp/demo \
  --input tmp/demo/exports/timeline.fcpxml \
  --commit \
  --format json \
  --format-error json
```

Classify changes:

```bash
go run ./cmd/vflow nle diff \
  --project tmp/demo \
  --import tmp/demo/imports/nle-import.json \
  --deliver file:tmp/demo/review/roundtrip-review.html \
  --format json \
  --format-error json
```

Apply safe or accepted changes:

```bash
go run ./cmd/vflow nle apply \
  --project tmp/demo \
  --input tmp/demo/imports/accepted-nle-changes.json \
  --commit \
  --format json \
  --format-error json
```

Guardrails:

- Every NLE export writes a sidecar mapping source frames to timeline frames.
- Ambiguous, identity-less, or high-risk editor changes are blocked or routed to review.
- Missing sidecar identity is not safe-merged.
- Speed changes, media replacement, color grades, complex effects, plugin effects, nested timelines, and keyframed transforms require review or are blocked.
- Resolve `.drp`, `.dra`, and `.drt` project packages are not treated as interchange files; export FCPXML, EDL, or OTIO first.

## Color And LUT Workflow

Review color:

```bash
go run ./cmd/vflow color review \
  --project tmp/demo \
  --input tmp/demo/renders/rough-preview.mp4 \
  --provider gemini \
  --live \
  --commit \
  --format json \
  --format-error json
```

Apply a LUT:

```bash
go run ./cmd/vflow color apply \
  --input tmp/demo/renders/rough-preview.mp4 \
  --lut fixtures/color/basic.cube \
  --deliver file:tmp/demo/renders/rough-preview-graded.mp4 \
  --commit \
  --format json \
  --format-error json
```

Color reports are adapter reports. They do not directly mutate canonical timeline/framing artifacts.

## Real Fixture Rules

The private CAIR-GA fixture used during development lives under ignored `work/`. Do not publish copied media or provider outputs.

For the prepared fixture:

- Use copied 4K source-camera clips under `media/source-4k/`.
- Do not use ATEM, ISO, program-output, finished-export, or edited 1080p files.
- Do not touch `/Volumes/Shams Drive` from this repo workflow.
- Do not write raw provider keys to the repo or thread.

## Documentation And Reports

Important docs:

- [AGENTS.md](AGENTS.md): repo rules for agents.
- [SKILL.md](SKILL.md): bundled vflow skill entrypoint.
- [skills/vflow-video-workflow/SKILL.md](skills/vflow-video-workflow/SKILL.md): workflow skill.
- [reports/vflow-implementation-report.md](reports/vflow-implementation-report.md): implementation history and proof commands.
- [reports/vflow-completion-audit.md](reports/vflow-completion-audit.md): current completion audit.
- [docs/research/current-docs-notes.md](docs/research/current-docs-notes.md): current docs research notes.
- [docs/research/gemini-files-api-upload-rejection-notes.md](docs/research/gemini-files-api-upload-rejection-notes.md): publishable Gemini Files API debugging article.

## Release And Upgrade

The repo includes GoReleaser configuration, a checksum-verifying install script, and an `upgrade` command.

Check public release metadata:

```bash
go run ./cmd/vflow upgrade --format json --format-error json
```

Stage an upgrade asset with explicit cache location:

```bash
go run ./cmd/vflow upgrade \
  --cache-dir tmp/upgrade-proof \
  --commit \
  --format json \
  --format-error json
```

Install the latest verified release into a PATH directory:

```bash
vflow upgrade \
  --install-dir "$HOME/.local/bin" \
  --commit \
  --format json \
  --format-error json
```

`upgrade --commit --install-dir` downloads the matching OS/architecture release archive, verifies it against `checksums.txt`, extracts the `vflow` binary, backs up any existing binary as `vflow.bak`, and atomically installs the replacement.

## Current Known Gap

`vflow` has structured and tested NLE export/import/diff/apply support across generated fixtures and representative editor-style changes. The remaining proof gap is exhaustive real-editor roundtrip validation from actual exported Resolve, Final Cut Pro, Premiere, Shotcut/Kdenlive MLT, and OTIO timelines.
