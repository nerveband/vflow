# vflow

`vflow` is an agent-native Go CLI for local-first video editing workflows. It turns a project folder into inspectable JSON contracts, deterministic preview renders, provider QA reports, color/LUT artifacts, and portable NLE exchange files with guarded roundtrip import.

The design goal is simple: agents can suggest edits, but `vflow` owns validation, canonical artifacts, frame math, safety gates, and writes.

## What Agents Use vflow For

Use `vflow` when an agent needs a reliable editing control plane instead of guessing from a chat transcript or mutating an editor project directly. The tool helps agents:

- Create and inspect a project folder with stable JSON artifacts.
- Probe media with `ffprobe` and record media facts in project artifacts.
- Import, create, search, align, and sync transcripts while keeping frame numbers canonical.
- Plan cleanup decisions, transcript cuts, timeline compiles, and preview renders with dry-run JSON before writing anything.
- Calibrate human-approved crop/zoom/reframe presets through a local browser UI, then compile framing lanes from preset IDs rather than invented crop boxes.
- Suggest finishing workflows for captions, audio, supers/cards, motion, SFX, and b-roll without becoming the renderer, compositor, or mixer.
- Verify external finishing outputs against canonical contracts, `brand.json`, timing anchors, and approved preset/style tokens, then route failures to `review/review-queue.json`.
- Render and verify previews with deterministic `ffmpeg` commands.
- Run live STT or Gemini QA only when the caller explicitly opts into provider quota.
- Export to NLE interchange formats and classify edited roundtrips without treating NLE files as source of truth.
- Deliver artifacts to stdout, files, or webhooks while preserving structured status.

The agent pattern is:

1. Run commands with `--format json --format-error json`.
2. Inspect `ok`, `command`, `data`, and artifact paths.
3. Keep mutating commands dry-run until the plan is acceptable.
4. Add `--commit` only when writes are intended.
5. Use canonical artifact paths and preset IDs in later commands.
6. For finishing work, run `vflow suggest <task>`, execute the recommended external tool yourself, then run `vflow verify <task>`.

## What You Can Rely On

`vflow` intentionally exposes a narrow contract:

- Canonical state lives in versioned JSON artifacts under the project folder.
- Command output uses `vflow-response/v1`; errors use `vflow-error/v1`.
- Mutating command metadata is listed by `vflow schema --validate`.
- Mutating workflows are dry-run by default and require `--commit` for writes.
- Live provider calls require `--live`; writing or costly live calls require `--commit`.
- Frame numbers are canonical. Seconds are readable derivatives.
- Framing compilers choose approved preset IDs; they do not invent crop rectangles.
- Finishing commands are control-plane commands. `suggest` recommends external tools and `verify` checks their outputs; neither command renders captions, composites graphics, mixes audio, or creates b-roll.
- NLE files are import/export adapters. Roundtrip changes are safe-merged, routed to review, or blocked.
- Secrets are read from runtime environment variables or external secret tools and are not written to repo artifacts.
- Local managed GUI sessions bind only to localhost and expose programmatic health/status/shutdown endpoints.

## When Not To Use vflow

Do not use `vflow` as:

- A general-purpose video editor or replacement for Resolve, Final Cut Pro, Premiere, Shotcut, or Kdenlive.
- A caption renderer, graphics compositor, motion renderer, audio mixer, SFX engine, or b-roll finder.
- A tool for generating unapproved crop boxes from an LLM suggestion.
- A way to mutate private media, provider outputs, or NLE project files without `--commit` and artifact review.
- A cloud job runner. The CLI is local-first; remote orchestration should wrap its JSON contracts.
- A secret store. Use environment variables, 1Password, Secret Gate, or another runtime secret system.
- A final authority for subjective creative judgment. It validates contracts and produces reviewable artifacts; humans still approve sensitive cuts, framing, color, and NLE roundtrips.

## Agent Vocabulary

Agents and humans often ask for the same operation with different words. `vflow` keeps one canonical command but exposes aliases for common intent terms. The canonical command still appears in output.

| Intent | Canonical command | Common aliases |
| --- | --- | --- |
| Start a crop/zoom/reframe calibration UI | `framing calibrate` | `framing crop`, `framing zoom`, `framing reframe`, `framing frame`, `framing crop-calibrate`, `framing zoom-calibrate`, `framing preset-calibrate` |
| Transcribe audio/video | `transcript create` | `transcript transcribe`, `transcript speech-to-text`, `transcript stt`, `transcribe stt` |
| Load transcript artifacts | `transcript import` | `transcript load-transcript`, `transcript ingest-transcript` |
| Align transcript words | `transcript align` | `transcript sync-transcript`, `transcript align-words`, `transcript word-align` |
| Add or inspect media | `media ingest`, `media probe` | `media add-media`, `media import-media`, `media inspect-media`, `media analyze-media`, `media metadata` |
| Build proxy media | `media proxy` | `media make-proxy`, `media create-proxy`, `media transcode-proxy` |
| Build timelines and framing lanes | `timeline compile`, `framing compile` | `timeline build-timeline`, `timeline make-timeline`, `timeline assemble`, `framing apply-framing`, `framing compile-framing`, `framing build-framing` |
| Render or verify preview media | `render preview`, `render verify` | `render make-preview`, `render render-sample`, `render verify-render`, `render check-render`, `render qa-render` |
| Exchange with an NLE | `nle export`, `nle import`, `nle diff` | `nle to-nle`, `nle export-nle`, `nle from-nle`, `nle import-nle`, `nle compare-nle`, `nle nle-compare` |
| List or deliver outputs | `artifacts list`, `artifacts deliver` | `outputs list`, `artifacts outputs`, `artifacts list-artifacts`, `artifacts publish-artifacts` |

Avoid terms like `auto-crop`, `detect-crop`, or `auto-reframe` unless a future command explicitly implements that behavior. In the current contract, crop/zoom/reframe calibration means a human approves crop presets through the GUI.

## Status

Current alpha capabilities:

- Structured JSON output and structured JSON errors for agent workflows.
- Command/schema introspection through `schema`, `agent-context`, `skill-path`, `doctor`, and `audit cli`.
- Dry-run by default for mutating work, with `--commit` required for writes and `--live` required for provider calls.
- Local project/media/transcript/cleanup/framing/timeline/render/color/NLE workflows.
- Finishing adapter registry in `doctor`, plus `suggest` and `verify` commands for captions, audio, supers/cards, motion, SFX, and b-roll.
- `brand.json` profile support for colors, fonts, logo variants, caption styles, layout IDs, loudness targets, safe margins, and consistency tokens.
- Managed localhost calibration UI for human-approved crop/zoom/reframe presets, with API endpoints for agent polling, artifact writes, and shutdown.
- ffprobe and ffmpeg adapters for media inspection, samples, proxy/range extraction, preview rendering, verification, transcript-cut rendering, and LUT application.
- Live STT adapters for OpenAI, ElevenLabs, Deepgram, AssemblyAI, Gladia, and Soniox.
- Gemini video QA through inline and Files API upload modes, including uploaded-file polling until `ACTIVE`.
- Color review and LUT workflows with Gemini enrichment when credentials are present.
- NLE export/import/diff/apply/accept support for Resolve-style FCPXML, FCPXML, EDL, OTIO, MLT, and Premiere XMEML where practical.
- Versioned artifact schemas for command output, project contracts, transcripts, timelines, QA reports, provider bakeoffs, NLE diffs/sidecars, render reports, and audit reports.
- Versioned finishing schemas for `brand.json`, caption cues, audio intent, supers/cards, motion ramps, SFX cues, and b-roll plans.

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
- Finishing tools are adapters. External tools may produce media, sidecars, or reports, but vflow owns the spec, recommendation, and verification result.

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
brand.json
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
- `brand.json`
- `transcript/words.json`
- `decisions/content-edl.json`
- `decisions/time-map.json`
- `calibration/framing-presets.json`
- `calibration/speaker-map.json`
- `decisions/framing-lane.json`
- `timeline/compiled-timeline.json`
- `timeline/vflow-timeline.json`
- `timeline/multicam-timeline.json`
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
vflow suggest captions
vflow verify captions
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

Finishing control plane:

```bash
vflow suggest captions --project ./my-video --format json --format-error json
vflow suggest audio --project ./my-video --format json --format-error json
vflow suggest supers --project ./my-video --format json --format-error json
vflow suggest motion --project ./my-video --format json --format-error json

# The agent runs the recommended external tool, then returns the contract/report to vflow.
vflow verify captions --project ./my-video --spec decisions/caption-cues.json --output artifacts/captions.srt --format json --format-error json
vflow verify audio --project ./my-video --spec decisions/audio-intent.json --output reports/audio-report.json --format json --format-error json
vflow verify supers --project ./my-video --spec decisions/super-cards.json --commit --format json --format-error json
vflow verify motion --project ./my-video --spec decisions/motion-ramp.json --output reports/motion-diff.json --commit --format json --format-error json
```

`suggest` returns the contract schema, a ranked adapter recommendation, a runnable invocation template, alternatives, detected capabilities, and missing-tool hints. For report-backed checks such as audio and motion, it also returns a `measurement_command` and `measurement_report` description so the report comes from a concrete tool pass rather than a hand-written claim. `verify` checks conformance and only writes review-queue failures when `--commit` is present.

Finishing contracts are intentionally tool-agnostic:

- `brand.json`: `vflow-brand/v1` tokens for colors, fonts, logo variants, caption styles, lower-third/card layout IDs, loudness targets, safe margins, and consistency tokens.
- `decisions/caption-cues.json`: `vflow-caption-cues/v1` cue timing anchored to `transcript/words.json`, style ID, and filler-clean intent. Timed caption output verification requires `transcript/words.json` to include the canonical `rate`; vflow routes missing rate to review instead of assuming 30fps.
- `decisions/audio-intent.json`: `vflow-audio-intent/v1` bed reference, duck target, loudness target, and speech-segment anchors.
- `decisions/super-cards.json`: `vflow-super-cards/v1` layout decisions tied to `brand.json` and the speaker map.
- `decisions/motion-ramp.json`: `vflow-motion-ramp/v1` ramp intent over approved framing preset IDs.
- `decisions/sfx-cues.json` and `decisions/broll-plan.json`: scaffold contracts for cue/overlay planning and future adapters.

Agent write contracts are inspectable before authoring payloads:

```bash
vflow schema command "cut create" --format json
vflow schema command "render transcript-cut" --format json
vflow schema command "media sync" --format json
```

For large external camera files, keep source media on the source drive and record intent in `source-media-review.json`:

```bash
vflow media ingest --source "/Volumes/Shams Drive/session-01-9mm.mp4" --reference --commit --format json
vflow media ingest --source "/Volumes/Shams Drive/session-01-9mm.mp4" --link --commit --format json
```

JSON payload flags accept `@file.json` so agents can fill schemas directly:

```bash
vflow cut create --ranges @decisions/ranges.json --sync-map calibration/media-sync-map.json --format json
vflow render transcript-cut --input @decisions/transcript-cut.json --output renders/social.mp4 --format json
```

Live transcript creation can request diarization and keyterm prompting, and defaults the frame rate from `source-media-review.json` when `--rate` is omitted:

```bash
vflow transcript create \
  --provider elevenlabs \
  --source "/Volumes/Shams Drive/session-01-9mm.mp4" \
  --diarize \
  --keyterms transcript/keyterms.txt \
  --live \
  --commit \
  --format json
```

Multicam sync can use transcript word anchors instead of low-confidence waveform correlation:

```bash
vflow media sync \
  --method transcript \
  --reference-source-id atem \
  --reference-words transcript/atem.words.json \
  --source-words 9mm=transcript/9mm.words.json,12mm=transcript/12mm.words.json \
  --commit \
  --format json
```

Waveform sync can sample multiple windows and median-vote the offset, which is safer for long event footage where one quiet or noisy section can mislead correlation:

```bash
vflow media sync \
  --reference-source-id atem \
  --sources atem=/Volumes/Shams\ Drive/proxies/atem.mp4,9mm=/Volumes/Shams\ Drive/proxies/9mm.mp4 \
  --sync-windows 3 \
  --commit \
  --format json
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
  --mode editorial \
  --prompt qa/editorial-prompt.md \
  --transcript transcript/words.json \
  --live \
  --commit \
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

Verify the export sidecar before handing the timeline to an editor:

```bash
go run ./cmd/vflow nle verify \
  --project tmp/demo \
  --sidecar tmp/demo/exports/sidecars/fcpxml-vflow-sidecar.json \
  --format json \
  --format-error json
```

Create a stacked-track multicam timeline from a sync map:

```bash
go run ./cmd/vflow multicam create \
  --project tmp/demo \
  --sync-map tmp/demo/decisions/media-sync-map.json \
  --duration-frames 900 \
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
- `timeline/vflow-timeline.json` is the canonical `vflow-timeline/v1` decision artifact. OTIO, FCPXML, XMEML, MLT, EDL, and Resolve-style FCPXML are adapters.
- OTIO export preserves stable clip IDs, linked audio/video IDs, track IDs, and vflow metadata where the target format can carry it.
- `nle verify` checks sidecar coverage, clip identity, frame mapping, missing markers, and trim drift against canonical timeline data when available.
- Ambiguous, identity-less, or high-risk editor changes are blocked or routed to review.
- Missing sidecar identity is not safe-merged.
- Speed changes, media replacement, color grades, complex effects, plugin effects, nested timelines, and keyframed transforms require review or are blocked.
- Resolve `.drp`, `.dra`, and `.drt` project packages are not treated as interchange files; export FCPXML, EDL, or OTIO first.
- Palmier remains an experimental external editor lane, not a first-party vflow backend. Recommended handoff is vflow canonical JSON -> OTIO/XML adapters -> Resolve/FCP/Premiere/Shotcut or another editor for finishing.

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

## Future Features

Planned or intentionally deferred features:

- Exhaustive real-editor roundtrip fixtures from Resolve, Final Cut Pro, Premiere, Shotcut/Kdenlive MLT, and OTIO timelines.
- Richer calibration ergonomics: keyboard nudging, preset duplication, aspect presets, better safe-zone presets, and visual diffing between crops.
- Optional review UIs for cleanup decisions, framing review queues, and NLE roundtrip decisions using the same localhost/session model as `framing calibrate`.
- More complete provenance chains linking transcripts, sync maps, source ranges, render reports, and NLE sidecars.
- Additional render targets and stricter platform presets for social formats.
- Deeper provider QA comparison reports that remain redacted and reproducible.
- Better install/upgrade reporting for local fleet or CI environments.

Future automation should preserve the current safety model: canonical JSON remains owned by `vflow`, writes stay commit-gated, and ambiguous creative decisions route to review instead of being silently invented.

## Current Known Gap

`vflow` has structured and tested NLE export/import/diff/apply support across generated fixtures and representative editor-style changes. The remaining proof gap is exhaustive real-editor roundtrip validation from actual exported Resolve, Final Cut Pro, Premiere, Shotcut/Kdenlive MLT, and OTIO timelines.
