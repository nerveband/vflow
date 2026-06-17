---
name: vflow-video-workflow
description: Run a vflow local-first video workflow from project setup through NLE roundtrip while preserving canonical JSON contracts.
---

# vflow Video Workflow

1. Inspect capabilities and command vocabulary:
   `vflow doctor --format json --format-error json`
   `vflow schema --validate --format json --format-error json`

2. Initialize or inspect the project:
   `vflow project init --path <project> --id <id> --commit --format json`
   or `vflow project get --project <project> --format json`.

3. Probe media before rendering:
   `vflow media probe --project <project> --commit --format json`.

4. Import transcripts into `transcript/words.json`:
   `vflow transcript import --project <project> --provider plain-text --input <file> --commit --format json`.

5. Apply reviewed cleanup decisions:
   `vflow cleanup apply --project <project> --input <delete_segments.json> --commit --format json`.

6. Calibrate or import human-approved framing presets:
   `vflow framing calibrate --project <project> --source media/source.mp4 --open=false --wait=false --format json --format-error json`.
   If a reviewed preset artifact already exists, use `vflow framing preset import --project <project> --input <framing-presets.json> --commit --format json`.

7. Compile frame-anchored framing and timeline artifacts:
   `vflow framing compile --project <project> --commit --format json`.
   `vflow timeline compile --project <project> --commit --format json`.

8. For finishing work, ask vflow for a contract and adapter recommendation before running external tools:
   `vflow suggest captions --project <project> --format json --format-error json`.
   `vflow suggest audio --project <project> --format json --format-error json`.
   `vflow suggest supers --project <project> --format json --format-error json`.
   `vflow suggest motion --project <project> --format json --format-error json`.
   The agent runs the recommended external tool. vflow owns the spec, recommendation, and verification, not rendering, compositing, or mixing.

9. Verify finishing outputs against canonical artifacts and route failures to review:
   `vflow verify captions --project <project> --spec decisions/caption-cues.json --output artifacts/captions.srt --format json --format-error json`.
   `vflow verify audio --project <project> --spec decisions/audio-intent.json --output reports/audio-report.json --format json --format-error json`.
   Add `--commit` only when review-queue writes are intended.

10. Render a preview only from copied/local project media:
   `vflow render preview --project <project> --commit --format json`.

11. Run QA/color provider calls only with runtime secrets:
   `vflow qa analyze --project <project> --provider gemini --live --commit --format json`.

12. Export to NLE with sidecar:
    `vflow nle export --project <project> --target fcpxml --commit --format json`.

Canonical artifacts live in JSON. `brand.json`, finishing specs, NLE files, Gemini reports, ffmpeg previews, and color notes are adapters, contracts, or reports, not the source of truth.

Useful aliases:

- `framing crop`, `framing zoom`, and `framing reframe` start the same managed calibration UI as `framing calibrate`.
- `transcript stt` and `transcript transcribe` map to transcript creation.
- `media inspect-media` maps to media probing.
- `timeline build-timeline` maps to timeline compilation.
- `render qa-render` maps to render verification.
- `nle to-nle`, `nle from-nle`, and `nle compare-nle` map to NLE export/import/diff.

Do not use vflow to invent crop boxes, store secrets, bypass `--commit`, render captions, composite supers/cards, mix audio, create motion graphics, or treat NLE project files as canonical state.
