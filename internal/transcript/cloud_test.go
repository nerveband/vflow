package transcript

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTranscribeElevenLabsPostsMultipartAndNormalizesWords(t *testing.T) {
	audioPath := writeTinyAudio(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/speech-to-text" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("model_id"); got != DefaultElevenLabsModel {
			t.Fatalf("model_id = %q", got)
		}
		if got := r.FormValue("timestamps_granularity"); got != "word" {
			t.Fatalf("timestamps_granularity = %q", got)
		}
		if _, _, err := r.FormFile("file"); err != nil {
			t.Fatalf("missing multipart file: %v", err)
		}
		_, _ = w.Write([]byte(`{"text":"hello world","words":[{"text":"hello","start":0,"end":0.5,"speaker_id":"speaker_1","logprob":-0.1},{"text":"world","start":0.5,"end":1.0,"speaker_id":"speaker_1","logprob":-0.2}]}`))
	}))
	defer server.Close()
	old := elevenLabsSpeechToTextURL
	elevenLabsSpeechToTextURL = server.URL + "/v1/speech-to-text"
	t.Cleanup(func() { elevenLabsSpeechToTextURL = old })

	got, err := TranscribeProvider(context.Background(), "elevenlabs", LiveTranscribeOptions{
		APIKey:        "test-key",
		AudioPath:     audioPath,
		Rate:          "30",
		SourceMediaID: "cam_a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "elevenlabs" || got.Model != DefaultElevenLabsModel || got.Text != "hello world" {
		t.Fatalf("unexpected transcription: %+v", got)
	}
	if len(got.Words.Words) != 2 || got.Words.Words[1].StartFrame != 15 || got.Words.Words[1].EndFrame != 30 {
		t.Fatalf("words not normalized to frames: %+v", got.Words.Words)
	}
	if got.Words.Words[0].SpeakerLabel != "speaker_1" {
		t.Fatalf("speaker not preserved: %+v", got.Words.Words[0])
	}
}

func TestTranscribeDeepgramPostsBinaryAndNormalizesWords(t *testing.T) {
	audioPath := writeTinyAudio(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/listen" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Token test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.URL.Query().Get("model"); got != DefaultDeepgramModel {
			t.Fatalf("model query = %q", got)
		}
		if got := r.URL.Query().Get("diarize"); got != "true" {
			t.Fatalf("diarize query = %q", got)
		}
		_, _ = w.Write([]byte(`{"metadata":{"request_id":"dg1"},"results":{"channels":[{"alternatives":[{"transcript":"hello deepgram","words":[{"word":"hello","start":0.1,"end":0.4,"confidence":0.98,"speaker":0},{"word":"deepgram","start":0.5,"end":0.9,"confidence":0.96,"speaker":1}]}]}]}}`))
	}))
	defer server.Close()
	old := deepgramListenURL
	deepgramListenURL = server.URL + "/v1/listen"
	t.Cleanup(func() { deepgramListenURL = old })

	got, err := TranscribeProvider(context.Background(), "deepgram", LiveTranscribeOptions{
		APIKey:        "test-key",
		AudioPath:     audioPath,
		Rate:          "30",
		SourceMediaID: "cam_a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.JobID != "dg1" || got.Text != "hello deepgram" {
		t.Fatalf("unexpected transcription: %+v", got)
	}
	if got.Words.Words[1].SpeakerLabel != "SPEAKER_01" {
		t.Fatalf("speaker not normalized: %+v", got.Words.Words[1])
	}
}

func TestTranscribeAssemblyAIUploadsSubmitsAndPolls(t *testing.T) {
	audioPath := writeTinyAudio(t)
	var submittedAudioURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/upload":
			if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
				t.Fatalf("Content-Type = %q", got)
			}
			_, _ = w.Write([]byte(`{"upload_url":"https://cdn.example/audio.mp4"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2/transcript":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			submittedAudioURL, _ = body["audio_url"].(string)
			_, _ = w.Write([]byte(`{"id":"aai1","status":"processing"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v2/transcript/aai1":
			_, _ = w.Write([]byte(`{"id":"aai1","status":"completed","text":"hello assembly","words":[{"text":"hello","start":0,"end":400,"confidence":0.9,"speaker":"A"},{"text":"assembly","start":400,"end":1000,"confidence":0.8,"speaker":"B"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	old := assemblyAIBaseURL
	assemblyAIBaseURL = server.URL + "/v2"
	t.Cleanup(func() { assemblyAIBaseURL = old })

	got, err := TranscribeProvider(context.Background(), "assemblyai", LiveTranscribeOptions{
		APIKey:        "test-key",
		AudioPath:     audioPath,
		Rate:          "30",
		SourceMediaID: "cam_a",
		PollInterval:  time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if submittedAudioURL != "https://cdn.example/audio.mp4" {
		t.Fatalf("submitted audio_url = %q", submittedAudioURL)
	}
	if got.JobID != "aai1" || got.Status != "completed" || got.Words.Words[1].EndFrame != 30 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestTranscribeGladiaUploadsInitiatesAndPolls(t *testing.T) {
	audioPath := writeTinyAudio(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-gladia-key"); got != "test-key" {
			t.Fatalf("x-gladia-key = %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/upload":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if _, _, err := r.FormFile("audio"); err != nil {
				t.Fatalf("missing upload audio: %v", err)
			}
			_, _ = w.Write([]byte(`{"audio_url":"https://api.gladia.io/file/test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2/pre-recorded":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"gladia1","result_url":"https://api.gladia.io/v2/pre-recorded/gladia1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v2/pre-recorded/gladia1":
			_, _ = w.Write([]byte(`{"id":"gladia1","status":"done","result":{"transcription":{"full_transcript":"hello gladia","utterances":[{"text":"hello gladia","speaker":1,"words":[{"word":"hello","start":0,"end":0.25,"confidence":0.92},{"word":"gladia","start":0.25,"end":0.75,"confidence":0.91}]}]}}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	old := gladiaBaseURL
	gladiaBaseURL = server.URL + "/v2"
	t.Cleanup(func() { gladiaBaseURL = old })

	got, err := TranscribeProvider(context.Background(), "gladia", LiveTranscribeOptions{
		APIKey:        "test-key",
		AudioPath:     audioPath,
		Rate:          "30",
		SourceMediaID: "cam_a",
		PollInterval:  time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.JobID != "gladia1" || got.Text != "hello gladia" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if len(got.Words.Words) != 2 || got.Words.Words[0].Text != "hello" {
		t.Fatalf("words not extracted: %+v", got.Words.Words)
	}
}

func TestTranscribeSonioxUploadsCreatesAndReadsTranscript(t *testing.T) {
	audioPath := writeTinyAudio(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/files":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if _, _, err := r.FormFile("file"); err != nil {
				t.Fatalf("missing upload file: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":"file1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/transcriptions":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"soniox1","status":"queued","model":"stt-async-v5"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/transcriptions/soniox1":
			_, _ = w.Write([]byte(`{"id":"soniox1","status":"completed","model":"stt-async-v5"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/transcriptions/soniox1/transcript":
			_, _ = w.Write([]byte(`{"id":"soniox1","text":"hello soniox","tokens":[{"text":"hello","start_ms":0,"end_ms":500,"confidence":0.9,"speaker":"spk1"},{"text":"soniox","start_ms":500,"end_ms":900,"confidence":0.9,"speaker":"spk1"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	old := sonioxBaseURL
	sonioxBaseURL = server.URL + "/v1"
	t.Cleanup(func() { sonioxBaseURL = old })

	got, err := TranscribeProvider(context.Background(), "soniox", LiveTranscribeOptions{
		APIKey:        "test-key",
		AudioPath:     audioPath,
		Rate:          "30",
		SourceMediaID: "cam_a",
		PollInterval:  time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.JobID != "soniox1" || !strings.Contains(got.Text, "soniox") {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.Words.Words[1].StartFrame != 15 {
		t.Fatalf("tokens not normalized: %+v", got.Words.Words)
	}
}

func writeTinyAudio(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tiny.mp4")
	if err := os.WriteFile(path, []byte("fake-media"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
