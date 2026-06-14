# Current Docs Notes

Checked on 2026-06-14.

## OpenAI Codex

- Codex documents `AGENTS.md` as durable project guidance loaded before work.
- Skills are directories with a required `SKILL.md` plus optional scripts, references, assets, and agents.
- Current subagent docs say subagents are enabled in current releases but spawned only when explicitly requested.

Sources:

- https://developers.openai.com/codex/guides/agents-md
- https://developers.openai.com/codex/skills
- https://developers.openai.com/codex/subagents

## Go CLI

- Cobra persistent flags are inherited by subcommands; local flags stay on one command.
- Cobra supports required, mutually exclusive, and grouped flags, which maps well to `--live --commit` and output-format validation.
- GoReleaser produces checksummed release artifacts; install scripts should verify checksums before installing binaries.

Sources:

- Context7 `/spf13/cobra`
- Context7 `/goreleaser/goreleaser`

## ffmpeg and ffprobe

- `ffprobe -output_format json` is the supported machine-readable probe path.
- `trim` keeps continuous subparts and can target time or frame boundaries; `setpts` is needed when resetting timestamps.
- `lut3d` supports `.cube` files and `tetrahedral` interpolation.

Sources:

- https://ffmpeg.org/ffprobe.html
- https://ffmpeg.org/ffmpeg-filters.html

## Gemini Video QA

- Google’s video understanding docs show uploading through the Files API for larger/reused videos and `generateContent` with uploaded file references.
- Gemini model docs currently list `gemini-3.5-flash` as stable and `gemini-3.1-pro-preview` as preview.
- Gemini release notes say Gemini 2.0 Flash models were shut down on 2026-06-01 and recommend `gemini-3.5-flash` or `gemini-3.1-flash-lite`.

Sources:

- https://ai.google.dev/gemini-api/docs/video-understanding
- https://ai.google.dev/gemini-api/docs/models
- https://ai.google.dev/gemini-api/docs/changelog

## STT Providers

- OpenAI file transcription supports `gpt-4o-transcribe`, `gpt-4o-mini-transcribe`, and `gpt-4o-transcribe-diarize`; uploads are limited to 25 MB.
- Soniox docs list `stt-async-v5` as active, with timestamps included by default.
- Gladia’s async STT product page documents word-level timestamps and diarization.

Sources:

- https://developers.openai.com/api/docs/guides/speech-to-text
- https://soniox.com/docs/stt/models
- https://soniox.com/docs/stt/concepts/timestamps
- https://www.gladia.io/product/async-transcription

## NLE Exchange

- FCPXML is Apple’s interchange format for media assets, edit decisions, and metadata.
- Premiere EDL export is CMX 3600 and works best for simple projects.
- OpenTimelineIO favors `.otio` JSON and supports additional adapters through plugin packages, including CMX 3600 and FCP XML.
- DaVinci Resolve scripting is Python/Lua based and requires Resolve running plus the Resolve scripting API paths.

Sources:

- https://developer.apple.com/documentation/professional-video-applications/fcpxml-reference
- https://helpx.adobe.com/premiere/desktop/render-and-export/export-files/export-a-project-as-an-edl-file.html
- https://opentimelineio.readthedocs.io/en/latest/tutorials/adapters.html
- https://resolvedevdoc.readthedocs.io/en/latest/API_intro.html
