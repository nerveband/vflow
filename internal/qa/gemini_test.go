package qa

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeGeminiModel(t *testing.T) {
	for input, want := range map[string]string{
		"3.1 pro":                "gemini-3.1-pro-preview",
		"gemini-3.1-pro-preview": "gemini-3.1-pro-preview",
		"gemini-3.5-flash":       "gemini-3.5-flash",
		"":                       "gemini-3.5-flash",
	} {
		got, err := NormalizeModel(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if got != want {
			t.Fatalf("%s: got %s want %s", input, got, want)
		}
	}
	if _, err := NormalizeModel("bad-model"); err == nil {
		t.Fatalf("expected bad model to fail")
	}
}

func TestDoctorMissingKeyIsCapabilityWarning(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	got, err := Doctor("gemini-3.5-flash", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.OK {
		t.Fatalf("missing key should not be ok")
	}
	if got.ErrorCode != "MISSING_API_KEY" {
		t.Fatalf("unexpected error code: %s", got.ErrorCode)
	}
}

func TestAnalyzeFileVideoUploadsThenUsesFileData(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "tiny.mp4")
	if err := os.WriteFile(videoPath, []byte("fake-video"), 0o644); err != nil {
		t.Fatal(err)
	}
	var server *httptest.Server
	startSeen := false
	uploadSeen := false
	generateSeen := false
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/upload/v1beta/files":
			startSeen = true
			if got := r.Header.Get("X-Goog-Upload-Protocol"); got != "resumable" {
				t.Fatalf("upload protocol = %q", got)
			}
			if got := r.Header.Get("X-Goog-Upload-Command"); got != "start" {
				t.Fatalf("upload command = %q", got)
			}
			w.Header().Set("X-Goog-Upload-URL", server.URL+"/upload-session")
			_, _ = w.Write([]byte(`{}`))
		case "/upload-session":
			uploadSeen = true
			if got := r.Header.Get("X-Goog-Upload-Command"); got != "upload, finalize" {
				t.Fatalf("final upload command = %q", got)
			}
			_, _ = w.Write([]byte(`{"file":{"name":"files/abc","uri":"https://files.example/video","mimeType":"video/mp4","state":"ACTIVE"}}`))
		case "/v1beta/models/gemini-3.5-flash:generateContent":
			generateSeen = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(body)
			if !strings.Contains(string(raw), `"file_uri":"https://files.example/video"`) {
				t.Fatalf("generateContent missing file_uri: %s", raw)
			}
			_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"ok\":true}"}]}}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	oldUpload, oldGenerate := geminiUploadURL, geminiGenerateBaseURL
	geminiUploadURL = server.URL + "/upload/v1beta/files"
	geminiGenerateBaseURL = server.URL + "/v1beta/models"
	t.Cleanup(func() {
		geminiUploadURL = oldUpload
		geminiGenerateBaseURL = oldGenerate
	})

	raw, file, err := AnalyzeFileVideo("test-key", "gemini-3.5-flash", videoPath, "review this")
	if err != nil {
		t.Fatal(err)
	}
	if !startSeen || !uploadSeen || !generateSeen {
		t.Fatalf("expected upload and generate paths, got start=%v upload=%v generate=%v", startSeen, uploadSeen, generateSeen)
	}
	if file.URI != "https://files.example/video" || !strings.Contains(raw, `"candidates"`) {
		t.Fatalf("unexpected response file=%+v raw=%s", file, raw)
	}
}
