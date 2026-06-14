package transcript

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTranscribeOpenAIPostsMultipartAndParsesText(t *testing.T) {
	audioPath := filepath.Join(t.TempDir(), "sample.mp4")
	if err := os.WriteFile(audioPath, []byte("fake-audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer test-key"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Fatal(err)
		}
		if got, want := r.FormValue("model"), DefaultOpenAITranscribeModel; got != want {
			t.Fatalf("model = %q, want %q", got, want)
		}
		if got, want := r.FormValue("response_format"), "json"; got != want {
			t.Fatalf("response_format = %q, want %q", got, want)
		}
		if _, _, err := r.FormFile("file"); err != nil {
			t.Fatalf("missing file part: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"text": "hello transcript"})
	}))
	defer server.Close()

	oldURL := openAITranscriptionsURL
	openAITranscriptionsURL = server.URL
	t.Cleanup(func() {
		openAITranscriptionsURL = oldURL
	})

	got, err := TranscribeOpenAI(context.Background(), "test-key", audioPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello transcript" || got.Model != DefaultOpenAITranscribeModel {
		t.Fatalf("unexpected transcription: %+v", got)
	}
}
