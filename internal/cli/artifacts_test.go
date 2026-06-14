package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactsDeliverRejectsExistingWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.json")
	dst := filepath.Join(dir, "out.json")
	if err := os.WriteFile(src, []byte(`{"new":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, errOut, code := runCLI(t, "artifacts", "deliver", "--input", src, "--deliver", "file:"+dst, "--commit", "--format", "json", "--format-error", "json")
	if code != 8 {
		t.Fatalf("expected external error, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "ARTIFACT_DELIVER_FAILED") {
		t.Fatalf("expected structured delivery error: %s", errOut)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != `{"old":true}` {
		t.Fatalf("destination changed without overwrite: %s", got)
	}
}

func TestArtifactsDeliverOverwriteCopiesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.json")
	dst := filepath.Join(dir, "out.json")
	if err := os.WriteFile(src, []byte(`{"new":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, "artifacts", "deliver", "--input", src, "--deliver", "file:"+dst, "--commit", "--overwrite", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, `"status": "delivered"`) {
		t.Fatalf("expected delivered status: %s", out)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != `{"new":true}` {
		t.Fatalf("destination not overwritten: %s", got)
	}
}

func TestArtifactsDeliverWebhookPostsWhenCommitted(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.json")
	if err := os.WriteFile(src, []byte(`{"new":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	out, errOut, code := runCLI(t, "artifacts", "deliver", "--input", src, "--deliver", "webhook:"+server.URL, "--commit", "--format", "json", "--format-error", "json")
	if code != 0 {
		t.Fatalf("expected success, got %d stderr=%s", code, errOut)
	}
	if !called {
		t.Fatalf("expected webhook to be called")
	}
	for _, want := range []string{`"status": "delivered"`, `"http_status": 202`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s in:\n%s", want, out)
		}
	}
}
