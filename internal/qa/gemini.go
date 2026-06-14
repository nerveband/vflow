package qa

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
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

var (
	geminiModelsURL       = "https://generativelanguage.googleapis.com/v1beta/models"
	geminiGenerateBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"
	geminiUploadURL       = "https://generativelanguage.googleapis.com/upload/v1beta/files"
	geminiFileBaseURL     = "https://generativelanguage.googleapis.com/v1beta"
)

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
	req, err := http.NewRequest(http.MethodGet, geminiModelsURL, nil)
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
	url := geminiGenerateURL(selected)
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
	url := geminiGenerateURL(selected)
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

type UploadedFile struct {
	Name     string `json:"name"`
	URI      string `json:"uri"`
	MIMEType string `json:"mime_type"`
	State    string `json:"state,omitempty"`
}

func UploadFile(apiKey, path string) (UploadedFile, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return UploadedFile{}, err
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		mimeType = "video/mp4"
	}
	startBody, _ := json.Marshal(map[string]any{"file": map[string]string{"display_name": filepath.Base(path)}})
	startReq, err := http.NewRequest(http.MethodPost, geminiUploadURL, bytes.NewReader(startBody))
	if err != nil {
		return UploadedFile{}, err
	}
	startReq.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	startReq.Header.Set("X-Goog-Upload-Protocol", "resumable")
	startReq.Header.Set("X-Goog-Upload-Command", "start")
	startReq.Header.Set("X-Goog-Upload-Header-Content-Length", fmt.Sprint(fileInfo.Size()))
	startReq.Header.Set("X-Goog-Upload-Header-Content-Type", mimeType)
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(startReq)
	if err != nil {
		return UploadedFile{}, err
	}
	defer startResp.Body.Close()
	startRaw, _ := io.ReadAll(startResp.Body)
	if startResp.StatusCode >= 300 {
		return UploadedFile{}, fmt.Errorf("Gemini file upload start returned %s: %s", startResp.Status, compactProviderBody(startRaw))
	}
	uploadURL := strings.TrimSpace(startResp.Header.Get("X-Goog-Upload-URL"))
	if uploadURL == "" {
		return UploadedFile{}, fmt.Errorf("Gemini file upload start did not return X-Goog-Upload-URL")
	}
	video, err := os.ReadFile(path)
	if err != nil {
		return UploadedFile{}, err
	}
	uploadReq, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(video))
	if err != nil {
		return UploadedFile{}, err
	}
	uploadReq.Header.Set("Content-Length", fmt.Sprint(len(video)))
	uploadReq.Header.Set("X-Goog-Upload-Offset", "0")
	uploadReq.Header.Set("X-Goog-Upload-Command", "upload, finalize")
	uploadResp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(uploadReq)
	if err != nil {
		return UploadedFile{}, err
	}
	defer uploadResp.Body.Close()
	uploadRaw, _ := io.ReadAll(uploadResp.Body)
	if uploadResp.StatusCode >= 300 {
		return UploadedFile{}, fmt.Errorf("Gemini file upload finalize returned %s: %s", uploadResp.Status, compactProviderBody(uploadRaw))
	}
	var parsed struct {
		File struct {
			Name     string `json:"name"`
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			State    string `json:"state"`
		} `json:"file"`
	}
	if err := json.Unmarshal(uploadRaw, &parsed); err != nil {
		return UploadedFile{}, err
	}
	if parsed.File.URI == "" {
		return UploadedFile{}, fmt.Errorf("Gemini file upload response missing file.uri")
	}
	return UploadedFile{Name: parsed.File.Name, URI: parsed.File.URI, MIMEType: firstNonEmpty(parsed.File.MIMEType, mimeType), State: parsed.File.State}, nil
}

func AnalyzeFileVideo(apiKey, model, videoPath, prompt string) (string, UploadedFile, error) {
	selected, err := NormalizeModel(model)
	if err != nil {
		return "", UploadedFile{}, err
	}
	uploaded, err := UploadFile(apiKey, videoPath)
	if err != nil {
		return "", UploadedFile{}, err
	}
	uploaded, err = WaitForFileActive(apiKey, uploaded, 3*time.Minute, 2*time.Second)
	if err != nil {
		return "", UploadedFile{}, err
	}
	body := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{
				{"file_data": map[string]string{"mime_type": uploaded.MIMEType, "file_uri": uploaded.URI}},
				{"text": prompt},
			},
		}},
		"generationConfig": map[string]string{"response_mime_type": "application/json"},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, geminiGenerateURL(selected), bytes.NewReader(raw))
	if err != nil {
		return "", UploadedFile{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return "", UploadedFile{}, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", UploadedFile{}, fmt.Errorf("Gemini generateContent returned %s: %s", resp.Status, compactProviderBody(out))
	}
	return string(out), uploaded, nil
}

func WaitForFileActive(apiKey string, file UploadedFile, timeout, interval time.Duration) (UploadedFile, error) {
	if file.State == "ACTIVE" || file.Name == "" {
		return file, nil
	}
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for {
		latest, err := getUploadedFile(apiKey, file.Name)
		if err != nil {
			return file, err
		}
		if latest.URI == "" {
			latest.URI = file.URI
		}
		if latest.MIMEType == "" {
			latest.MIMEType = file.MIMEType
		}
		switch latest.State {
		case "ACTIVE":
			return latest, nil
		case "FAILED":
			return latest, fmt.Errorf("Gemini uploaded file %s entered FAILED state", file.Name)
		}
		if time.Now().Add(interval).After(deadline) {
			return latest, fmt.Errorf("Gemini uploaded file %s did not become ACTIVE before timeout; current state %q", file.Name, latest.State)
		}
		time.Sleep(interval)
	}
}

func getUploadedFile(apiKey, name string) (UploadedFile, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(geminiFileBaseURL, "/")+"/"+strings.TrimLeft(name, "/"), nil)
	if err != nil {
		return UploadedFile{}, err
	}
	req.Header.Set("x-goog-api-key", strings.TrimSpace(apiKey))
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return UploadedFile{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return UploadedFile{}, fmt.Errorf("Gemini file get returned %s: %s", resp.Status, compactProviderBody(raw))
	}
	var parsed struct {
		Name     string `json:"name"`
		URI      string `json:"uri"`
		MIMEType string `json:"mimeType"`
		State    string `json:"state"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return UploadedFile{}, err
	}
	return UploadedFile{Name: parsed.Name, URI: parsed.URI, MIMEType: parsed.MIMEType, State: parsed.State}, nil
}

func SanitizeProviderResponse(raw string) json.RawMessage {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		quoted, _ := json.Marshal(raw)
		return json.RawMessage(quoted)
	}
	stripTransientGeminiFields(value)
	out, err := json.Marshal(value)
	if err != nil {
		quoted, _ := json.Marshal(raw)
		return json.RawMessage(quoted)
	}
	return json.RawMessage(out)
}

func stripTransientGeminiFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "thoughtSignature" {
				delete(typed, key)
				continue
			}
			stripTransientGeminiFields(child)
		}
	case []any:
		for _, child := range typed {
			stripTransientGeminiFields(child)
		}
	}
}

func geminiGenerateURL(model string) string {
	return strings.TrimRight(geminiGenerateBaseURL, "/") + "/" + model + ":generateContent"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
