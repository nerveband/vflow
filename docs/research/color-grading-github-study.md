# Color Grading And LUT Research Checklist

Checked on 2026-06-14.

## Findings To Borrow

- FFmpeg `lut3d` directly supports `.cube` files and `interp=tetrahedral`; vflow uses this for deterministic preview grading.
- OpenColorIO/OCIO remains the better long-term path for color-managed transforms and LUT baking; vflow treats this as later integration.
- Resolve scripting can automate LUT/CDL operations, but Resolve state is not canonical in vflow.
- OpenTimeline/NLE workflows should keep color as a separate report/lane because EDL/FCPXML sidecars are safer than assuming portable grade semantics.

## What To Avoid

- Do not let Gemini or another provider directly mutate timeline/color artifacts.
- Do not roundtrip complex Resolve/Premiere color effects into canonical JSON in v1.
- Do not assume `.cube` means a correct color space; preserve LUT path/hash and require human review.

## Sources

- https://ffmpeg.org/ffmpeg-filters.html
- https://opencolorio.readthedocs.io/en/latest/tutorials/baking_luts.html
- https://resolvedevdoc.readthedocs.io/en/latest/API_intro.html
- https://github.com/samuelgursky/davinci-resolve-mcp
- https://github.com/AcademySoftwareFoundation/OpenColorIO
