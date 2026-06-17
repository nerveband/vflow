package transcript

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultElevenLabsModel = "scribe_v2"
	DefaultDeepgramModel   = "nova-3"
	DefaultAssemblyAIModel = "default"
	DefaultGladiaModel     = "pre-recorded-v2"
	DefaultSonioxModel     = "stt-async-v5"
)

var (
	elevenLabsSpeechToTextURL = "https://api.elevenlabs.io/v1/speech-to-text"
	deepgramListenURL         = "https://api.deepgram.com/v1/listen"
	assemblyAIBaseURL         = "https://api.assemblyai.com/v2"
	gladiaBaseURL             = "https://api.gladia.io/v2"
	sonioxBaseURL             = "https://api.soniox.com/v1"
)

type LiveTranscribeOptions struct {
	APIKey        string
	AudioPath     string
	Model         string
	Rate          string
	SourceMediaID string
	Diarize       bool
	Keyterms      []string
	PollInterval  time.Duration
}

type ProviderTranscription struct {
	Provider string          `json:"provider"`
	Model    string          `json:"model,omitempty"`
	Audio    string          `json:"audio,omitempty"`
	JobID    string          `json:"job_id,omitempty"`
	Status   string          `json:"status,omitempty"`
	Text     string          `json:"text"`
	Words    Words           `json:"words"`
	Raw      json.RawMessage `json:"raw,omitempty"`
}

type TimedWord struct {
	Text       string
	Start      float64
	End        float64
	Confidence float64
	Speaker    string
}

func TranscribeProvider(ctx context.Context, provider string, opts LiveTranscribeOptions) (ProviderTranscription, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.APIKey) == "" {
		return ProviderTranscription{}, fmt.Errorf("%s API key is not set", provider)
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	switch provider {
	case "openai":
		tx, err := TranscribeOpenAI(ctx, opts.APIKey, opts.AudioPath, opts.Model)
		if err != nil {
			return ProviderTranscription{}, err
		}
		return providerTranscriptionFromText(tx.Provider, tx.Model, tx.Audio, "", "completed", tx.Text, tx.Raw, opts), nil
	case "elevenlabs":
		return transcribeElevenLabs(ctx, opts)
	case "deepgram":
		return transcribeDeepgram(ctx, opts)
	case "assemblyai":
		return transcribeAssemblyAI(ctx, opts)
	case "gladia":
		return transcribeGladia(ctx, opts)
	case "soniox":
		return transcribeSoniox(ctx, opts)
	default:
		return ProviderTranscription{}, fmt.Errorf("unsupported live transcript provider %q", provider)
	}
}

func transcribeElevenLabs(ctx context.Context, opts LiveTranscribeOptions) (ProviderTranscription, error) {
	model := firstString(opts.Model, DefaultElevenLabsModel)
	fields := map[string][]string{
		"model_id":               {model},
		"timestamps_granularity": {"word"},
	}
	if opts.Diarize {
		fields["diarize"] = []string{"true"}
	}
	for _, keyterm := range opts.Keyterms {
		keyterm = strings.TrimSpace(keyterm)
		if keyterm != "" {
			fields["keyterms"] = append(fields["keyterms"], keyterm)
		}
	}
	raw, err := postMultipartMulti(ctx, endpointFromEnv("VFLOW_ELEVENLABS_STT_URL", elevenLabsSpeechToTextURL), "xi-api-key", opts.APIKey, "file", opts.AudioPath, fields)
	if err != nil {
		return ProviderTranscription{}, err
	}
	var parsed struct {
		Text  string `json:"text"`
		Words []struct {
			Text      string  `json:"text"`
			Start     float64 `json:"start"`
			End       float64 `json:"end"`
			SpeakerID string  `json:"speaker_id"`
			Logprob   float64 `json:"logprob"`
		} `json:"words"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ProviderTranscription{}, err
	}
	words := make([]TimedWord, 0, len(parsed.Words))
	for _, word := range parsed.Words {
		confidence := 0.0
		if word.Logprob != 0 {
			confidence = math.Exp(word.Logprob)
			if confidence > 1 {
				confidence = 1
			}
		}
		words = append(words, TimedWord{Text: word.Text, Start: word.Start, End: word.End, Speaker: word.SpeakerID, Confidence: confidence})
	}
	return providerTranscriptionFromTimedWords("elevenlabs", model, opts.AudioPath, "", "completed", parsed.Text, words, raw, opts), nil
}

func transcribeDeepgram(ctx context.Context, opts LiveTranscribeOptions) (ProviderTranscription, error) {
	model := firstString(opts.Model, DefaultDeepgramModel)
	endpoint, err := url.Parse(endpointFromEnv("VFLOW_DEEPGRAM_LISTEN_URL", deepgramListenURL))
	if err != nil {
		return ProviderTranscription{}, err
	}
	query := endpoint.Query()
	query.Set("model", model)
	query.Set("smart_format", "true")
	query.Set("diarize", "true")
	endpoint.RawQuery = query.Encode()
	file, err := os.Open(opts.AudioPath)
	if err != nil {
		return ProviderTranscription{}, err
	}
	defer file.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), file)
	if err != nil {
		return ProviderTranscription{}, err
	}
	req.Header.Set("Authorization", "Token "+strings.TrimSpace(opts.APIKey))
	req.Header.Set("Content-Type", "application/octet-stream")
	raw, err := doProviderRequest(req)
	if err != nil {
		return ProviderTranscription{}, err
	}
	var parsed struct {
		Metadata struct {
			RequestID string `json:"request_id"`
		} `json:"metadata"`
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
					Words      []struct {
						Word       string  `json:"word"`
						Start      float64 `json:"start"`
						End        float64 `json:"end"`
						Confidence float64 `json:"confidence"`
						Speaker    any     `json:"speaker"`
					} `json:"words"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ProviderTranscription{}, err
	}
	text := ""
	var words []TimedWord
	if len(parsed.Results.Channels) > 0 && len(parsed.Results.Channels[0].Alternatives) > 0 {
		alt := parsed.Results.Channels[0].Alternatives[0]
		text = alt.Transcript
		words = make([]TimedWord, 0, len(alt.Words))
		for _, word := range alt.Words {
			words = append(words, TimedWord{
				Text:       word.Word,
				Start:      word.Start,
				End:        word.End,
				Confidence: word.Confidence,
				Speaker:    normalizeSpeaker(word.Speaker),
			})
		}
	}
	return providerTranscriptionFromTimedWords("deepgram", model, opts.AudioPath, parsed.Metadata.RequestID, "completed", text, words, raw, opts), nil
}

func transcribeAssemblyAI(ctx context.Context, opts LiveTranscribeOptions) (ProviderTranscription, error) {
	baseURL := baseEndpointFromEnv("VFLOW_ASSEMBLYAI_BASE_URL", assemblyAIBaseURL)
	uploadRaw, err := postBinary(ctx, baseURL+"/upload", "Authorization", opts.APIKey, opts.AudioPath)
	if err != nil {
		return ProviderTranscription{}, err
	}
	var upload struct {
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(uploadRaw, &upload); err != nil {
		return ProviderTranscription{}, err
	}
	if upload.UploadURL == "" {
		return ProviderTranscription{}, fmt.Errorf("AssemblyAI upload response missing upload_url")
	}
	body := map[string]any{
		"audio_url":      upload.UploadURL,
		"speaker_labels": true,
		"punctuate":      true,
		"format_text":    true,
	}
	model := firstString(opts.Model, DefaultAssemblyAIModel)
	if opts.Model != "" {
		body["speech_model"] = opts.Model
	}
	raw, err := postJSON(ctx, baseURL+"/transcript", "Authorization", opts.APIKey, body)
	if err != nil {
		return ProviderTranscription{}, err
	}
	return pollAssemblyAI(ctx, opts, model, baseURL, raw)
}

func pollAssemblyAI(ctx context.Context, opts LiveTranscribeOptions, model, baseURL string, raw json.RawMessage) (ProviderTranscription, error) {
	for {
		tx, status, id, err := parseAssemblyAI(raw, opts, model)
		if err != nil {
			return ProviderTranscription{}, err
		}
		if status == "completed" {
			return tx, nil
		}
		if status == "error" || status == "failed" {
			return ProviderTranscription{}, fmt.Errorf("AssemblyAI transcription %s failed", id)
		}
		if id == "" {
			return ProviderTranscription{}, fmt.Errorf("AssemblyAI transcription response missing id")
		}
		if err := sleepContext(ctx, opts.PollInterval); err != nil {
			return ProviderTranscription{}, err
		}
		next, err := getJSON(ctx, baseURL+"/transcript/"+url.PathEscape(id), "Authorization", opts.APIKey)
		if err != nil {
			return ProviderTranscription{}, err
		}
		raw = next
	}
}

func parseAssemblyAI(raw json.RawMessage, opts LiveTranscribeOptions, model string) (ProviderTranscription, string, string, error) {
	var parsed struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Text   string `json:"text"`
		Words  []struct {
			Text       string  `json:"text"`
			Start      float64 `json:"start"`
			End        float64 `json:"end"`
			Confidence float64 `json:"confidence"`
			Speaker    any     `json:"speaker"`
		} `json:"words"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ProviderTranscription{}, "", "", err
	}
	words := make([]TimedWord, 0, len(parsed.Words))
	for _, word := range parsed.Words {
		words = append(words, TimedWord{
			Text:       word.Text,
			Start:      word.Start / 1000,
			End:        word.End / 1000,
			Confidence: word.Confidence,
			Speaker:    normalizeSpeaker(word.Speaker),
		})
	}
	tx := providerTranscriptionFromTimedWords("assemblyai", model, opts.AudioPath, parsed.ID, parsed.Status, parsed.Text, words, raw, opts)
	return tx, parsed.Status, parsed.ID, nil
}

func transcribeGladia(ctx context.Context, opts LiveTranscribeOptions) (ProviderTranscription, error) {
	baseURL := baseEndpointFromEnv("VFLOW_GLADIA_BASE_URL", gladiaBaseURL)
	uploadRaw, err := postMultipart(ctx, baseURL+"/upload", "x-gladia-key", opts.APIKey, "audio", opts.AudioPath, nil)
	if err != nil {
		return ProviderTranscription{}, err
	}
	var upload struct {
		AudioURL string `json:"audio_url"`
	}
	if err := json.Unmarshal(uploadRaw, &upload); err != nil {
		return ProviderTranscription{}, err
	}
	if upload.AudioURL == "" {
		return ProviderTranscription{}, fmt.Errorf("Gladia upload response missing audio_url")
	}
	raw, err := postJSON(ctx, baseURL+"/pre-recorded", "x-gladia-key", opts.APIKey, map[string]any{
		"audio_url":   upload.AudioURL,
		"diarization": true,
	})
	if err != nil {
		return ProviderTranscription{}, err
	}
	var started struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &started); err != nil {
		return ProviderTranscription{}, err
	}
	if started.ID == "" {
		return ProviderTranscription{}, fmt.Errorf("Gladia transcription response missing id")
	}
	for {
		next, err := getJSON(ctx, baseURL+"/pre-recorded/"+url.PathEscape(started.ID), "x-gladia-key", opts.APIKey)
		if err != nil {
			return ProviderTranscription{}, err
		}
		var status struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(next, &status); err != nil {
			return ProviderTranscription{}, err
		}
		if status.Status == "done" {
			text, words := extractProviderTextAndWords(next)
			return providerTranscriptionFromTimedWords("gladia", firstString(opts.Model, DefaultGladiaModel), opts.AudioPath, started.ID, status.Status, text, words, next, opts), nil
		}
		if status.Status == "error" {
			return ProviderTranscription{}, fmt.Errorf("Gladia transcription %s failed", started.ID)
		}
		if err := sleepContext(ctx, opts.PollInterval); err != nil {
			return ProviderTranscription{}, err
		}
	}
}

func transcribeSoniox(ctx context.Context, opts LiveTranscribeOptions) (ProviderTranscription, error) {
	baseURL := baseEndpointFromEnv("VFLOW_SONIOX_BASE_URL", sonioxBaseURL)
	uploadRaw, err := postMultipart(ctx, baseURL+"/files", "Authorization", "Bearer "+strings.TrimSpace(opts.APIKey), "file", opts.AudioPath, nil)
	if err != nil {
		return ProviderTranscription{}, err
	}
	var upload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(uploadRaw, &upload); err != nil {
		return ProviderTranscription{}, err
	}
	if upload.ID == "" {
		return ProviderTranscription{}, fmt.Errorf("Soniox upload response missing id")
	}
	model := firstString(opts.Model, DefaultSonioxModel)
	raw, err := postJSON(ctx, baseURL+"/transcriptions", "Authorization", "Bearer "+strings.TrimSpace(opts.APIKey), map[string]any{
		"model":                          model,
		"file_id":                        upload.ID,
		"enable_speaker_diarization":     true,
		"enable_language_identification": true,
	})
	if err != nil {
		return ProviderTranscription{}, err
	}
	var started struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &started); err != nil {
		return ProviderTranscription{}, err
	}
	if started.ID == "" {
		return ProviderTranscription{}, fmt.Errorf("Soniox transcription response missing id")
	}
	for {
		statusRaw, err := getJSON(ctx, baseURL+"/transcriptions/"+url.PathEscape(started.ID), "Authorization", "Bearer "+strings.TrimSpace(opts.APIKey))
		if err != nil {
			return ProviderTranscription{}, err
		}
		var status struct {
			Status       string `json:"status"`
			ErrorMessage string `json:"error_message"`
		}
		if err := json.Unmarshal(statusRaw, &status); err != nil {
			return ProviderTranscription{}, err
		}
		if status.Status == "completed" {
			transcriptRaw, err := getJSON(ctx, baseURL+"/transcriptions/"+url.PathEscape(started.ID)+"/transcript", "Authorization", "Bearer "+strings.TrimSpace(opts.APIKey))
			if err != nil {
				return ProviderTranscription{}, err
			}
			text, words := extractProviderTextAndWords(transcriptRaw)
			return providerTranscriptionFromTimedWords("soniox", model, opts.AudioPath, started.ID, status.Status, text, words, transcriptRaw, opts), nil
		}
		if status.Status == "error" {
			return ProviderTranscription{}, fmt.Errorf("Soniox transcription %s failed: %s", started.ID, status.ErrorMessage)
		}
		if err := sleepContext(ctx, opts.PollInterval); err != nil {
			return ProviderTranscription{}, err
		}
	}
}

func postMultipart(ctx context.Context, endpoint, keyHeader, keyValue, fileField, filePath string, fields map[string]string) (json.RawMessage, error) {
	multi := map[string][]string{}
	for key, value := range fields {
		multi[key] = []string{value}
	}
	return postMultipartMulti(ctx, endpoint, keyHeader, keyValue, fileField, filePath, multi)
}

func postMultipartMulti(ctx context.Context, endpoint, keyHeader, keyValue, fileField, filePath string, fields map[string][]string) (json.RawMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fileField, filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	for key, values := range fields {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return nil, err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(keyHeader, strings.TrimSpace(keyValue))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return doProviderRequest(req)
}

func postBinary(ctx context.Context, endpoint, keyHeader, keyValue, filePath string) (json.RawMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, file)
	if err != nil {
		return nil, err
	}
	req.Header.Set(keyHeader, strings.TrimSpace(keyValue))
	req.Header.Set("Content-Type", "application/octet-stream")
	return doProviderRequest(req)
}

func postJSON(ctx context.Context, endpoint, keyHeader, keyValue string, body map[string]any) (json.RawMessage, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set(keyHeader, strings.TrimSpace(keyValue))
	req.Header.Set("Content-Type", "application/json")
	return doProviderRequest(req)
}

func getJSON(ctx context.Context, endpoint, keyHeader, keyValue string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(keyHeader, strings.TrimSpace(keyValue))
	return doProviderRequest(req)
}

func doProviderRequest(req *http.Request) (json.RawMessage, error) {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s returned %s: %s", req.Method, req.URL, resp.Status, compactProviderBody(raw))
	}
	return json.RawMessage(raw), nil
}

func providerTranscriptionFromText(provider, model, audio, jobID, status, text string, raw json.RawMessage, opts LiveTranscribeOptions) ProviderTranscription {
	words, _ := Import("plain-text", []byte(text), ImportOptions{SourceMediaID: opts.SourceMediaID, Rate: opts.Rate, FramesPerWord: 15})
	for i := range words.Words {
		words.Words[i].Provider = provider
	}
	return ProviderTranscription{Provider: provider, Model: model, Audio: filepath.ToSlash(audio), JobID: jobID, Status: status, Text: text, Words: words, Raw: raw}
}

func providerTranscriptionFromTimedWords(provider, model, audio, jobID, status, text string, timed []TimedWord, raw json.RawMessage, opts LiveTranscribeOptions) ProviderTranscription {
	if strings.TrimSpace(text) == "" {
		text = strings.TrimSpace(joinTimedWords(timed))
	}
	if len(timed) == 0 {
		return providerTranscriptionFromText(provider, model, audio, jobID, status, text, raw, opts)
	}
	words := Words{Version: "vflow-words/v1", SourceMediaID: firstString(opts.SourceMediaID, "source"), Rate: firstString(opts.Rate, "30000/1001")}
	for _, timedWord := range timed {
		clean := strings.TrimSpace(timedWord.Text)
		if clean == "" {
			continue
		}
		start := secondsToFrames(timedWord.Start, words.Rate)
		end := secondsToFrames(timedWord.End, words.Rate)
		if end <= start {
			end = start + 1
		}
		words.Words = append(words.Words, Word{
			ID:           fmt.Sprintf("w_%06d", len(words.Words)+1),
			Text:         clean,
			SpeakerLabel: timedWord.Speaker,
			StartFrame:   start,
			EndFrame:     end,
			Confidence:   timedWord.Confidence,
			Provider:     provider,
		})
	}
	return ProviderTranscription{Provider: provider, Model: model, Audio: filepath.ToSlash(audio), JobID: jobID, Status: status, Text: text, Words: words, Raw: raw}
}

func secondsToFrames(seconds float64, rate string) int64 {
	if seconds < 0 {
		seconds = 0
	}
	return int64(math.Round(seconds * frameRate(rate)))
}

func frameRate(rate string) float64 {
	rate = strings.TrimSpace(rate)
	if rate == "" {
		return 30000.0 / 1001.0
	}
	if before, after, ok := strings.Cut(rate, "/"); ok {
		num, nerr := strconv.ParseFloat(strings.TrimSpace(before), 64)
		den, derr := strconv.ParseFloat(strings.TrimSpace(after), 64)
		if nerr == nil && derr == nil && den != 0 {
			return num / den
		}
	}
	if val, err := strconv.ParseFloat(rate, 64); err == nil && val > 0 {
		return val
	}
	return 30000.0 / 1001.0
}

func extractProviderTextAndWords(raw json.RawMessage) (string, []TimedWord) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", nil
	}
	var words []TimedWord
	extractWords(value, "", &words)
	return extractBestText(value), words
}

func extractWords(value any, inheritedSpeaker string, out *[]TimedWord) {
	switch node := value.(type) {
	case []any:
		for _, item := range node {
			extractWords(item, inheritedSpeaker, out)
		}
	case map[string]any:
		speaker := inheritedSpeaker
		if val, ok := node["speaker"]; ok {
			speaker = normalizeSpeaker(val)
		}
		if val, ok := node["speaker_id"]; ok {
			speaker = normalizeSpeaker(val)
		}
		if word, ok := timedWordFromMap(node, speaker); ok {
			*out = append(*out, word)
		}
		for key, child := range node {
			if key == "speaker" || key == "speaker_id" {
				continue
			}
			extractWords(child, speaker, out)
		}
	}
}

func timedWordFromMap(node map[string]any, speaker string) (TimedWord, bool) {
	if _, hasWords := node["words"]; hasWords {
		return TimedWord{}, false
	}
	if _, hasTokens := node["tokens"]; hasTokens {
		return TimedWord{}, false
	}
	text := stringValue(node["word"])
	if text == "" {
		text = stringValue(node["text"])
	}
	if strings.TrimSpace(text) == "" {
		return TimedWord{}, false
	}
	start, okStart := timeValueSeconds(node, "start", "start_time", "startTime")
	if !okStart {
		start, okStart = timeValueMillis(node, "start_ms", "startTimeMs", "start_time_ms")
	}
	end, okEnd := timeValueSeconds(node, "end", "end_time", "endTime")
	if !okEnd {
		end, okEnd = timeValueMillis(node, "end_ms", "endTimeMs", "end_time_ms")
	}
	if !okStart || !okEnd {
		return TimedWord{}, false
	}
	confidence, _ := floatValue(node["confidence"])
	return TimedWord{Text: text, Start: start, End: end, Confidence: confidence, Speaker: speaker}, true
}

func extractBestText(value any) string {
	best := ""
	var walk func(any)
	walk = func(node any) {
		switch val := node.(type) {
		case []any:
			for _, child := range val {
				walk(child)
			}
		case map[string]any:
			for _, key := range []string{"full_transcript", "transcript", "text"} {
				if text := strings.TrimSpace(stringValue(val[key])); len(text) > len(best) {
					best = text
				}
			}
			for _, child := range val {
				walk(child)
			}
		}
	}
	walk(value)
	return best
}

func timeValueSeconds(node map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := floatValue(node[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func timeValueMillis(node map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := floatValue(node[key]); ok {
			return value / 1000, true
		}
	}
	return 0, false
}

func floatValue(value any) (float64, bool) {
	switch val := value.(type) {
	case float64:
		return val, true
	case json.Number:
		out, err := val.Float64()
		return out, err == nil
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		out, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return out, err == nil
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	switch val := value.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return ""
	}
}

func normalizeSpeaker(value any) string {
	switch val := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(val)
	case float64:
		return fmt.Sprintf("SPEAKER_%02d", int(val))
	case int:
		return fmt.Sprintf("SPEAKER_%02d", val)
	case int64:
		return fmt.Sprintf("SPEAKER_%02d", val)
	default:
		return strings.TrimSpace(fmt.Sprint(val))
	}
}

func joinTimedWords(words []TimedWord) string {
	parts := make([]string, 0, len(words))
	for _, word := range words {
		if strings.TrimSpace(word.Text) != "" {
			parts = append(parts, strings.TrimSpace(word.Text))
		}
	}
	return strings.Join(parts, " ")
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func endpointFromEnv(envName, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return fallback
}

func baseEndpointFromEnv(envName, fallback string) string {
	return strings.TrimRight(endpointFromEnv(envName, fallback), "/")
}
