---
name: vflow
description: Use when working with local video projects through the vflow CLI: media probe, transcript import, cleanup, framing, timeline compilation, rendering, QA, color review, and NLE exchange.
---

# vflow Skill

Use `vflow agent-context --format json` before editing a project so you have the current command surface, safety rules, provider matrix, and artifact contracts. Use `vflow schema --validate --format json --format-error json` when you need the discoverable command list, including aliases.

Preferred workflow:

```bash
vflow project init --path ./demo --id demo --commit --format json
vflow media probe --project ./demo --format json --commit
vflow transcript import --project ./demo --provider generic-words --input transcript/words.json --commit --format json
vflow cleanup apply --project ./demo --input decisions/delete_segments.json --commit --format json
vflow framing calibrate --project ./demo --source media/source.mp4 --open=false --wait=false --format json --format-error json
vflow framing compile --project ./demo --commit --format json
vflow timeline compile --project ./demo --commit --format json
vflow suggest captions --project ./demo --format json --format-error json
vflow verify captions --project ./demo --spec decisions/caption-cues.json --output artifacts/captions.srt --format json --format-error json
vflow render preview --project ./demo --commit --format json
vflow qa analyze --project ./demo --provider gemini --live --commit --format json
vflow nle export --project ./demo --target fcpxml --commit --format json
```

Agent rules:

- Always prefer `--format json --format-error json`.
- Keep writes dry-run until the returned plan/artifact paths are acceptable; add `--commit` only for intended writes.
- Treat `vflow` JSON artifacts as source of truth. NLE files, provider reports, and renders are adapters or evidence.
- Use `vflow suggest <task>` and `vflow verify <task>` for finishing work. vflow owns contracts, brand/timing/frame-math truth, recommendations, and verification; external tools own rendering, compositing, mixing, SFX, and b-roll execution.
- For crop/zoom/reframe work, use `framing calibrate` or its aliases (`framing crop`, `framing zoom`, `framing reframe`) to collect human-approved presets. Do not invent crop boxes.
- Use alias terms when helpful, but expect canonical command names in output.

Never commit provider secrets, private media, copied `.env` files, or local source-drive paths.
