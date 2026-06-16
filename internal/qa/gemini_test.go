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
		"2.5 flash":               "gemini-2.5-flash",
		"3.1 pro":                 "gemini-3.1-pro-preview",
		"gemini-2.5-flash":        "gemini-2.5-flash",
		"models/gemini-2.5-flash": "gemini-2.5-flash",
		"gemini-3.1-pro-preview":  "gemini-3.1-pro-preview",
		"gemini-3.5-flash":        "gemini-3.5-flash",
		"gemini-9.9-exp":          "gemini-9.9-exp",
		"video":                   "gemini-3-flash-preview",
		"":                        "gemini-3.5-flash",
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

func TestAPIKeyFromNamedEnv(t *testing.T) {
	t.Setenv("GEMINI_CANDIDATE_1", "test-key")
	key, source, err := APIKeyFromNamedEnv("GEMINI_CANDIDATE_1")
	if err != nil {
		t.Fatal(err)
	}
	if key != "test-key" || source != "env:GEMINI_CANDIDATE_1" {
		t.Fatalf("unexpected key source: key=%q source=%q", key, source)
	}
	if _, _, err := APIKeyFromNamedEnv("BAD-NAME"); err == nil {
		t.Fatalf("expected invalid env name to fail")
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
	getSeen := false
	generateSeen := false
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/upload/v1beta/files":
			startSeen = true
			if got := r.URL.Query().Get("key"); got != "test-key" {
				t.Fatalf("upload start missing api key query: %q", got)
			}
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
			_, _ = w.Write([]byte(`{"file":{"name":"files/abc","uri":"https://files.example/video","mimeType":"video/mp4","state":"PROCESSING"}}`))
		case "/v1beta/files/abc":
			getSeen = true
			if got := r.URL.Query().Get("key"); got != "test-key" {
				t.Fatalf("file get missing api key query: %q", got)
			}
			_, _ = w.Write([]byte(`{"name":"files/abc","uri":"https://files.example/video","mimeType":"video/mp4","state":"ACTIVE"}`))
		case "/v1beta/models/gemini-3-flash-preview:generateContent":
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
	oldFileBase := geminiFileBaseURL
	geminiUploadURL = server.URL + "/upload/v1beta/files"
	geminiGenerateBaseURL = server.URL + "/v1beta/models"
	geminiFileBaseURL = server.URL + "/v1beta"
	t.Cleanup(func() {
		geminiUploadURL = oldUpload
		geminiGenerateBaseURL = oldGenerate
		geminiFileBaseURL = oldFileBase
	})

	raw, file, err := AnalyzeFileVideo("test-key", "video", videoPath, "review this")
	if err != nil {
		t.Fatal(err)
	}
	if !startSeen || !uploadSeen || !getSeen || !generateSeen {
		t.Fatalf("expected upload, get, and generate paths, got start=%v upload=%v get=%v generate=%v", startSeen, uploadSeen, getSeen, generateSeen)
	}
	if file.URI != "https://files.example/video" || !strings.Contains(raw, `"candidates"`) {
		t.Fatalf("unexpected response file=%+v raw=%s", file, raw)
	}
}

func TestSanitizeProviderResponseStripsThoughtSignatures(t *testing.T) {
	raw := `{"candidates":[{"content":{"parts":[{"text":"{}","thoughtSignature":"opaque"}]}}]}`
	got := SanitizeProviderResponse(raw)
	if strings.Contains(string(got), "thoughtSignature") || strings.Contains(string(got), "opaque") {
		t.Fatalf("expected thought signature to be removed: %s", got)
	}
	if !strings.Contains(string(got), `"text":"{}"`) {
		t.Fatalf("expected response content to remain: %s", got)
	}
}
