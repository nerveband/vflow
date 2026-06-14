package output

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

type DeliveryResult struct {
	Status string `json:"status"`
	Input  string `json:"input"`
	Output string `json:"output"`
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
