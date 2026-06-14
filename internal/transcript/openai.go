package transcript

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultOpenAITranscribeModel = "gpt-4o-transcribe"

var openAITranscriptionsURL = "https://api.openai.com/v1/audio/transcriptions"

type OpenAITranscription struct {
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Audio    string          `json:"audio"`
	Text     string          `json:"text"`
	Raw      json.RawMessage `json:"raw,omitempty"`
}

func TranscribeOpenAI(ctx context.Context, apiKey, audioPath, model string) (OpenAITranscription, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return OpenAITranscription{}, fmt.Errorf("OPENAI_API_KEY is not set")
	}
	if model == "" {
		model = DefaultOpenAITranscribeModel
	}

	file, err := os.Open(audioPath)
	if err != nil {
		return OpenAITranscription{}, err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return OpenAITranscription{}, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return OpenAITranscription{}, err
	}
	if err := writer.WriteField("model", model); err != nil {
		return OpenAITranscription{}, err
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return OpenAITranscription{}, err
	}
	if err := writer.Close(); err != nil {
		return OpenAITranscription{}, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAITranscriptionsURL, &body)
	if err != nil {
		return OpenAITranscription{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return OpenAITranscription{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return OpenAITranscription{}, fmt.Errorf("OpenAI transcription returned %s: %s", resp.Status, compactProviderBody(raw))
	}

	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return OpenAITranscription{}, err
	}
	return OpenAITranscription{
		Provider: "openai",
		Model:    model,
		Audio:    filepath.ToSlash(audioPath),
		Text:     parsed.Text,
		Raw:      json.RawMessage(raw),
	}, nil
}

func compactProviderBody(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 400 {
		return text[:400] + "..."
	}
	return text
}
