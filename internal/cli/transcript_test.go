package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranscriptImportWritesCanonicalWords(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "transcript_test", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("Ali: Bismillah welcome\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCLI(t, "transcript", "import", "--project", dir, "--provider", "plain-text", "--input", input, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "transcript", "words.json")); err != nil {
		t.Fatalf("expected words.json: %v", err)
	}
}

func TestTranscriptImportRejectsInvalidCanonicalWords(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bad-words.json")
	raw := `{"version":"vflow-words/v1","source_media_id":"source","rate":"30000/1001","words":[{"id":"w_000001","text":"bad","start_frame":30,"end_frame":30,"confidence":0.9,"provider":"generic-words"}]}`
	if err := os.WriteFile(input, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runCLI(t, "transcript", "import", "--project", dir, "--provider", "generic-words", "--input", input, "--commit", "--format", "json", "--format-error", "json")
	if code == 0 {
		t.Fatalf("expected invalid transcript failure, stdout=%s stderr=%s", out, errOut)
	}
	for _, want := range []string{`"code": "TRANSCRIPT_IMPORT_FAILED"`, "end_frame must be greater than start_frame"} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("expected %q in stderr:\n%s", want, errOut)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "transcript", "words.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no words.json to be written, stat err=%v", err)
	}
}

func TestTranscriptSearchLocalUsesProjectIndex(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VFLOW_INDEX_PATH", filepath.Join(t.TempDir(), "index.sqlite"))
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "search_cli", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("Ali: bismillah zakat sadaqa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runCLI(t, "transcript", "import", "--project", dir, "--provider", "plain-text", "--input", input, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("transcript import failed: %s", errOut)
	}
	_, errOut, code = runCLI(t, "project", "index", "--path", dir, "--commit", "--format", "json")
	if code != 0 {
		t.Fatalf("project index failed: %s", errOut)
	}

	out, errOut, code := runCLI(t, "transcript", "search", "--project", dir, "--query", "zakat", "--data-source", "local", "--format", "json")
	if code != 0 {
		t.Fatalf("transcript search failed: code=%d stdout=%s stderr=%s", code, out, errOut)
	}
	for _, want := range []string{`"data_source": "local"`, `"text": "zakat"`, `"project_id": "search_cli"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("search output missing %s in:\n%s", want, out)
		}
	}
}

func TestTranscriptCreateLiveDeepgramWritesWordsAndReport(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runCLI(t, "project", "init", "--path", dir, "--id", "deepgram_cli", "--commit", "--format", "json"); code != 0 {
		t.Fatalf("project init failed")
	}
	source := filepath.Join(dir, "media", "source.mp4")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("fake-media"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Token test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"metadata":{"request_id":"dg-cli"},"results":{"channels":[{"alternatives":[{"transcript":"hello cli","words":[{"word":"hello","start":0,"end":0.5,"confidence":0.9},{"word":"cli","start":0.5,"end":1,"confidence":0.9}]}]}]}}`))
	}))
	defer server.Close()
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	t.Setenv("VFLOW_DEEPGRAM_LISTEN_URL", server.URL+"/v1/listen")

	out, errOut, code := runCLI(t, "transcript", "create", "--project", dir, "--provider", "deepgram", "--source", source, "--live", "--commit", "--timeout", "5s", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stdout=%s stderr=%s", code, out, errOut)
	}
	for _, want := range []string{`"provider": "deepgram"`, `"word_count": 2`, `"job_id": "dg-cli"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s in:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "transcript", "words.json")); err != nil {
		t.Fatalf("expected canonical words: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "transcript", "deepgram-transcription.json")); err != nil {
		t.Fatalf("expected provider report: %v", err)
	}
}

func TestTranscriptBakeoffLiveSkipsMissingOptionalKeysAndWritesReport(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "media", "source.mp4")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("fake-media"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ELEVENLABS_API_KEY", "")

	out, errOut, code := runCLI(t, "transcript", "bakeoff", "--project", dir, "--source", source, "--providers", "elevenlabs,local", "--live", "--commit", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected missing keys to be recorded, got %d stderr=%s", code, errOut)
	}
	for _, want := range []string{`"status": "skipped_missing_key"`, `"status": "local_import_only"`, `provider-bakeoff.json`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s in:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "reports", "provider-bakeoff.json")); err != nil {
		t.Fatalf("expected bakeoff report: %v", err)
	}
}
