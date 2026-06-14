---
name: vflow
description: Use when working with local video projects through the vflow CLI: media probe, transcript import, cleanup, framing, timeline compilation, rendering, QA, color review, and NLE exchange.
---

# vflow Skill

Use `vflow agent-context --format json` before editing a project so you have the current command surface, safety rules, provider matrix, and artifact contracts.

Preferred workflow:

```bash
vflow project init --path ./demo --id demo --commit --format json
vflow media probe --project ./demo --format json --commit
vflow transcript import --project ./demo --provider generic-words --input transcript/words.json --commit --format json
vflow cleanup apply --project ./demo --input decisions/delete_segments.json --commit --format json
vflow framing preset import --project ./demo --input calibration/framing-presets.json --commit --format json
vflow timeline compile --project ./demo --commit --format json
vflow render preview --project ./demo --commit --format json
vflow qa analyze --project ./demo --provider gemini --live --commit --format json
vflow nle export --project ./demo --target fcpxml --commit --format json
```

Never commit provider secrets, private media, copied `.env` files, or local source-drive paths.
