# vflow

`vflow` is an agent-native Go CLI for local-first video editing contracts,
preview rendering, provider QA, and NLE exchange.

The first alpha favors structured JSON, explicit safety flags, and portable
project artifacts over editor-specific state.

## First Commands

```bash
go test ./...
go run ./cmd/vflow --help
go run ./cmd/vflow version --format json
```

## Working Slice

```bash
go run ./cmd/vflow project init --path tmp/live-demo --id live_demo --commit --format json
go run ./cmd/vflow media probe --project tmp/live-demo --source tmp/live-demo/media/source.mp4 --commit --format json
go run ./cmd/vflow transcript create --project tmp/live-demo --provider openai --source tmp/live-demo/media/source.mp4 --live --commit --format json
go run ./cmd/vflow render preview --project tmp/live-demo --source tmp/live-demo/media/source.mp4 --commit --format json
go run ./cmd/vflow color apply --input tmp/live-demo/renders/rough-preview.mp4 --lut fixtures/color/basic.cube --deliver file:tmp/live-demo/renders/rough-preview-graded.mp4 --commit --format json
go run ./cmd/vflow nle export --project tmp/live-demo --target fcpxml --deliver file:tmp/live-demo/exports/timeline.fcpxml --commit --format json
```

See `reports/vflow-implementation-report.md` for proof commands, live provider
results, and copied-fixture validation notes.
