package qa

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultFastModel = "gemini-3.5-flash"
	DeepReviewModel  = "gemini-3.1-pro-preview"
)

type ModelInfo struct {
	Name string `json:"name"`
}

type DoctorResult struct {
	Provider        string   `json:"provider"`
	OK              bool     `json:"ok"`
	ErrorCode       string   `json:"error_code,omitempty"`
	KeyPresent      bool     `json:"key_present"`
	KeySource       string   `json:"key_source,omitempty"`
	SelectedModel   string   `json:"selected_model"`
	ModelAvailable  bool     `json:"model_available"`
	AvailableModels []string `json:"available_models,omitempty"`
	Live            bool     `json:"live"`
}

func NormalizeModel(model string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "", "fast", "flash", DefaultFastModel:
		return DefaultFastModel, nil
	case "3.1 pro", "gemini 3.1 pro", "gemini-3.1-pro", DeepReviewModel:
		return DeepReviewModel, nil
	default:
		return "", fmt.Errorf("unknown Gemini model %q; valid alternatives: %s, %s", model, DefaultFastModel, DeepReviewModel)
	}
}

func Doctor(model string, live bool) (DoctorResult, error) {
	selected, err := NormalizeModel(model)
	if err != nil {
		return DoctorResult{}, err
	}
	key, source := APIKeyFromEnv()
	res := DoctorResult{Provider: "gemini", OK: key != "", KeyPresent: key != "", SelectedModel: selected, Live: live}
	if key != "" {
		res.KeySource = source
	} else {
		res.ErrorCode = "MISSING_API_KEY"
	}
	if !live || key == "" {
		res.ModelAvailable = selected == DefaultFastModel || selected == DeepReviewModel
		return res, nil
	}
	models, err := ListModels(key)
	if err != nil {
		return res, err
	}
	res.AvailableModels = models
	for _, available := range models {
		if strings.HasSuffix(available, selected) || available == selected {
			res.ModelAvailable = true
			break
		}
	}
	return res, nil
}

func APIKeyFromEnv() (string, string) {
	for _, key := range []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value, "env:" + key
		}
	}
	return "", ""
}

func ListModels(apiKey string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://generativelanguage.googleapis.com/v1beta/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini models endpoint returned %s: %s", resp.Status, compactProviderBody(raw))
	}
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(body.Models))
	for _, model := range body.Models {
		models = append(models, model.Name)
	}
	return models, nil
}

func AnalyzeTextOnly(apiKey, model, prompt string) (string, error) {
	selected, err := NormalizeModel(model)
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"contents":         []map[string]any{{"parts": []map[string]string{{"text": prompt}}}},
		"generationConfig": map[string]string{"response_mime_type": "application/json"},
	}
	raw, _ := json.Marshal(body)
	url := "https://generativelanguage.googleapis.com/v1beta/models/" + selected + ":generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini generateContent returned %s: %s", resp.Status, compactProviderBody(out))
	}
	return string(out), nil
}

func AnalyzeInlineVideo(apiKey, model, videoPath, prompt string) (string, error) {
	selected, err := NormalizeModel(model)
	if err != nil {
		return "", err
	}
	video, err := os.ReadFile(videoPath)
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{
				{"text": prompt},
				{"inlineData": map[string]string{"mimeType": "video/mp4", "data": base64.StdEncoding.EncodeToString(video)}},
			},
		}},
		"generationConfig": map[string]string{"response_mime_type": "application/json"},
	}
	raw, _ := json.Marshal(body)
	url := "https://generativelanguage.googleapis.com/v1beta/models/" + selected + ":generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini generateContent returned %s: %s", resp.Status, compactProviderBody(out))
	}
	return string(out), nil
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
