# Gemini Files API Upload Rejections: What Actually Breaks

Date: 2026-06-16

This note captures the debugging lessons from wiring Gemini video QA into `vflow`, where plain `generateContent` calls worked but Gemini Files API uploads were rejected or failed later in the video-analysis path.

The short version: do not treat "API key invalid" as proof that the key itself is bad. In the Files API upload flow, auth placement, MIME metadata, file state, and `generateContent` payload shape can all produce errors that look like key or provider rejection problems.

## The Working Mental Model

Gemini file prompting has two different phases:

1. Upload media to the Files API.
2. Reference the uploaded file URI from `generateContent`.

For video, this is usually the right path when the request is large, the video is longer, or the same media will be reused. Google's current Files API guide says the Files API stores files separately from prompt input, supports media reuse, stores files for 48 hours, supports up to 20 GB per project, and has a 2 GB per-file limit.

The official Go SDK path is:

```go
client, err := genai.NewClient(ctx, &genai.ClientConfig{
	APIKey:  os.Getenv("GEMINI_API_KEY"),
	Backend: genai.BackendGeminiAPI,
})

file, err := client.Files.UploadFromPath(ctx, "video.mp4", &genai.UploadFileConfig{
	MIMEType: "video/mp4",
})

resp, err := client.Models.GenerateContent(ctx, "gemini-3.5-flash", []*genai.Content{
	genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromURI(file.URI, file.MIMEType),
		genai.NewPartFromText("Describe this video."),
	}, genai.RoleUser),
}, nil)
```

The lower-level REST flow is:

1. Start a resumable upload session at `/upload/v1beta/files`.
2. Send the file bytes to the returned `X-Goog-Upload-URL`.
3. Poll `files.get` until the uploaded file is `ACTIVE`.
4. Call `models/{model}:generateContent` with `file_data`.

## The Rejection Pattern We Hit

`vflow` originally had Gemini text/model calls working while Files API uploads failed. That made it tempting to diagnose the issue as a rotated, expired, or invalid key.

The real issue was narrower: the resumable upload lifecycle is more sensitive to how auth is attached than the normal `generateContent` request. In live probing, the upload-start and file-get paths worked reliably when the key was attached as a `?key=...` query parameter, while `generateContent` continued to work with `x-goog-api-key`.

That distinction matters because Google's docs currently show both forms in different places:

- The Files guide REST example shows `x-goog-api-key` for the upload-start request.
- The Files API reference shell examples show the upload-start URL as `/upload/v1beta/files?key=${GEMINI_API_KEY}`.
- SDKs hide this detail, so SDK users may never see the mismatch.

The practical fix in a direct REST client is:

- Use `?key=...` for Files API upload-start.
- Use `?key=...` for `files.get` polling.
- Use `x-goog-api-key` for `generateContent`.
- Keep all key values in runtime environment variables, not source files or logs.

## Correct REST Shape

Upload start:

```bash
MIME_TYPE="video/mp4"
NUM_BYTES="$(wc -c < video.mp4)"

curl "https://generativelanguage.googleapis.com/upload/v1beta/files?key=${GEMINI_API_KEY}" \
  -D upload-header.tmp \
  -H "X-Goog-Upload-Protocol: resumable" \
  -H "X-Goog-Upload-Command: start" \
  -H "X-Goog-Upload-Header-Content-Length: ${NUM_BYTES}" \
  -H "X-Goog-Upload-Header-Content-Type: ${MIME_TYPE}" \
  -H "Content-Type: application/json" \
  -d "{\"file\":{\"display_name\":\"video.mp4\"}}"
```

Upload bytes:

```bash
UPLOAD_URL="$(grep -i 'x-goog-upload-url:' upload-header.tmp | cut -d' ' -f2- | tr -d '\r')"

curl "${UPLOAD_URL}" \
  -H "Content-Length: ${NUM_BYTES}" \
  -H "Content-Type: ${MIME_TYPE}" \
  -H "X-Goog-Upload-Offset: 0" \
  -H "X-Goog-Upload-Command: upload, finalize" \
  --data-binary "@video.mp4" \
  > file-info.json
```

Poll metadata:

```bash
FILE_NAME="$(jq -r '.file.name' file-info.json)"

curl "https://generativelanguage.googleapis.com/v1beta/${FILE_NAME}?key=${GEMINI_API_KEY}" \
  > file-state.json
```

Wait until the state is `ACTIVE`. Do not send a `PROCESSING` file into `generateContent`.

Generate with file data:

```bash
FILE_URI="$(jq -r '.uri // .file.uri' file-state.json)"
MIME_TYPE="$(jq -r '.mimeType // .file.mimeType // "video/mp4"' file-state.json)"

curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash:generateContent" \
  -H "x-goog-api-key: ${GEMINI_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"contents\": [{
      \"parts\": [
        {\"file_data\": {\"mime_type\": \"${MIME_TYPE}\", \"file_uri\": \"${FILE_URI}\"}},
        {\"text\": \"Describe this video.\"}
      ]
    }]
  }"
```

## Common Causes Of Upload Rejection

### 1. The Key Is Not Actually In The Process

This is still the first check. Shell restarts, GUI app launches, `sudo`, background jobs, and task runners can all drop environment variables.

Verify presence without printing the key:

```bash
test -n "${GEMINI_API_KEY:-}" && echo "GEMINI_API_KEY present" || echo "missing"
```

Then verify model access with a cheap model-list or text request before uploading media.

### 2. Auth Works For Models But Not Upload

If `models.list` or text `generateContent` works but upload-start fails with `API_KEY_INVALID`, test both documented auth placements for the upload-start request:

```bash
# Header style shown in some guide examples.
curl "https://generativelanguage.googleapis.com/upload/v1beta/files" \
  -H "x-goog-api-key: ${GEMINI_API_KEY}" ...

# Query style shown in API reference examples and used by vflow's proven REST path.
curl "https://generativelanguage.googleapis.com/upload/v1beta/files?key=${GEMINI_API_KEY}" ...
```

In `vflow`, query auth was the reliable upload-start and file-get path.

### 3. Missing Or Wrong MIME Type

Do not let video become `application/octet-stream`. Set the upload content type explicitly:

```text
X-Goog-Upload-Header-Content-Type: video/mp4
Content-Type: video/mp4
```

Then pass the same MIME type in `file_data`.

### 4. Using The File Before Processing Finishes

The Files API has processing states:

- `PROCESSING`: file is stored but not ready for inference.
- `ACTIVE`: file is ready for inference.
- `FAILED`: processing failed.

For video, polling matters. Treat `PROCESSING` as not ready, not as success.

### 5. Wrong `generateContent` Part Shape

For REST, the file reference part should look like this:

```json
{
  "file_data": {
    "mime_type": "video/mp4",
    "file_uri": "https://generativelanguage.googleapis.com/v1beta/files/..."
  }
}
```

In the Go SDK, prefer `genai.NewPartFromURI(file.URI, file.MIMEType)` or `genai.NewPartFromFile(*file)`.

### 6. Model Name Or Availability Drift

Do not hardcode an old model allowlist. For this project, `gemini-3.5-flash` was live and listed by the model endpoint on 2026-06-16. The model catalog changes, so a good CLI should accept explicit `gemini-*` model names and expose a doctor command that can list availability.

### 7. File Size And Request Strategy

As of the checked docs, use Files API for total requests larger than 100 MB, with PDF called out separately at 50 MB in the guide. The Files API reference also documents 20 GB of stored files per project and a 2 GB per-file limit. For small videos, inline upload can be a useful fallback path, but the Files API is the right test for production-like video QA.

## A Good Diagnostic Ladder

Use this order so you do not confuse unrelated failures:

1. Confirm the key is present in the current process without printing it.
2. Run a live model-list or text-only `generateContent` request.
3. Start a tiny Files API upload with explicit MIME type.
4. Confirm upload finalization returns `file.name`, `file.uri`, and `file.mimeType`.
5. Poll `files.get` until `ACTIVE`.
6. Call `generateContent` with `file_data`.
7. If upload-start fails, compare header auth versus query auth.
8. If generation fails, inspect whether the file is still `PROCESSING`, the MIME type is wrong, or the payload uses `fileData`/`file_data` incorrectly for the chosen SDK/REST API.

## What `vflow` Changed

The `vflow` REST adapter now:

- Resolves Gemini keys from runtime env only.
- Uses `?key=...` for Files API upload-start.
- Uses `?key=...` for Files API metadata polling.
- Uses `x-goog-api-key` for `generateContent`.
- Forces MIME detection with a `video/mp4` fallback.
- Polls uploaded files until `ACTIVE`.
- Sends `file_data` with both `mime_type` and `file_uri`.
- Sanitizes transient Gemini fields such as `thoughtSignature` before writing committed reports.

Live proof command:

```bash
go run ./cmd/vflow qa analyze \
  --project work/live-provider-proof/gemini \
  --render work/live-provider-proof/gemini/renders/rough-preview.mp4 \
  --provider gemini \
  --model gemini-3.5-flash \
  --upload files \
  --live \
  --commit \
  --timeout 5m \
  --format json \
  --format-error json
```

Observed result:

- `gemini-3.5-flash` was available.
- The video uploaded through the Files API.
- The uploaded file reached `ACTIVE`.
- Gemini returned a video QA response with video token usage metadata.
- The report was written as a structured `vflow-gemini-video-qa/v1` artifact.

## Sources

- Google Gemini Files API guide: https://ai.google.dev/gemini-api/docs/files
- Google Gemini Files API reference: https://ai.google.dev/api/files
- Google Gen AI Go SDK docs via Context7: `/googleapis/go-genai`, checked 2026-06-16
- `vflow` implementation proof: `internal/qa/gemini.go`, `internal/qa/gemini_test.go`, and `reports/vflow-implementation-report.md`
