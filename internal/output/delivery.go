package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DeliveryResult struct {
	Status     string `json:"status"`
	Input      string `json:"input"`
	Output     string `json:"output,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
}

func DeliverFile(src, dst string, overwrite bool) (DeliveryResult, error) {
	if src == "" || dst == "" {
		return DeliveryResult{}, errors.New("source and destination are required")
	}
	if !overwrite {
		if _, err := os.Stat(dst); err == nil {
			return DeliveryResult{}, os.ErrExist
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return DeliveryResult{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return DeliveryResult{}, err
	}

	in, err := os.Open(src)
	if err != nil {
		return DeliveryResult{}, err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".vflow-deliver-*")
	if err != nil {
		return DeliveryResult{}, err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return DeliveryResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return DeliveryResult{}, err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return DeliveryResult{}, err
	}
	committed = true
	return DeliveryResult{Status: "delivered", Input: src, Output: dst}, nil
}

func DeliverWebhook(src, target string) (DeliveryResult, error) {
	if src == "" || target == "" {
		return DeliveryResult{}, errors.New("source and webhook target are required")
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return DeliveryResult{}, err
	}
	var artifact any
	if err := json.Unmarshal(raw, &artifact); err != nil {
		artifact = string(raw)
	}
	body := map[string]any{
		"schema_version": "vflow-artifact-delivery/v1",
		"input":          src,
		"artifact":       artifact,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return DeliveryResult{}, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimSpace(target), bytes.NewReader(encoded))
	if err != nil {
		return DeliveryResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "vflow-artifact-delivery")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return DeliveryResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return DeliveryResult{}, errors.New(resp.Status)
	}
	return DeliveryResult{Status: "delivered", Input: src, Output: target, HTTPStatus: resp.StatusCode}, nil
}
