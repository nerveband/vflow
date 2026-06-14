---
name: vflow-video-workflow
description: Run a vflow local-first video workflow from project setup through NLE roundtrip while preserving canonical JSON contracts.
---

# vflow Video Workflow

1. Inspect capabilities:
   `vflow doctor --format json`

2. Initialize or inspect the project:
   `vflow project init --path <project> --id <id> --commit --format json`
   or `vflow project get --project <project> --format json`.

3. Probe media before rendering:
   `vflow media probe --project <project> --commit --format json`.

4. Import transcripts into `transcript/words.json`:
   `vflow transcript import --project <project> --provider plain-text --input <file> --commit --format json`.

5. Apply reviewed cleanup decisions:
   `vflow cleanup apply --project <project> --input <delete_segments.json> --commit --format json`.

6. Import and validate framing presets:
   `vflow framing preset import --project <project> --input <framing-presets.json> --commit --format json`.

7. Compile frame-anchored timeline artifacts:
   `vflow timeline compile --project <project> --commit --format json`.

8. Render a preview only from copied/local project media:
   `vflow render preview --project <project> --commit --format json`.

9. Run QA/color provider calls only with runtime secrets:
   `vflow qa analyze --project <project> --provider gemini --live --commit --format json`.

10. Export to NLE with sidecar:
    `vflow nle export --project <project> --target fcpxml --commit --format json`.

Canonical artifacts live in JSON. NLE files, Gemini reports, ffmpeg previews, and color notes are adapters or reports, not the source of truth.
