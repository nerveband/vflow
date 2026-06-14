package framing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Preset struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Type   string `json:"type"`
	CropPX Rect   `json:"crop_px"`
}

type Presets struct {
	Version      string   `json:"version"`
	SourceWidth  int      `json:"source_width"`
	SourceHeight int      `json:"source_height"`
	TargetAspect string   `json:"target_aspect"`
	Presets      []Preset `json:"presets"`
}

func ParsePresets(raw []byte) (Presets, error) {
	var presets Presets
	if err := json.Unmarshal(raw, &presets); err != nil {
		return Presets{}, err
	}
	return presets, nil
}

func (p Presets) Validate() error {
	ids := map[string]bool{}
	for _, preset := range p.Presets {
		if preset.ID == "" {
			return fmt.Errorf("preset id is required")
		}
		if strings.HasPrefix(preset.ID, "SPEAKER_") {
			return fmt.Errorf("preset %s uses diarization label; use stable person/preset ids", preset.ID)
		}
		if ids[preset.ID] {
			return fmt.Errorf("duplicate preset id %s", preset.ID)
		}
		ids[preset.ID] = true
		if preset.CropPX.W <= 0 || preset.CropPX.H <= 0 {
			return fmt.Errorf("preset %s has invalid crop size", preset.ID)
		}
		if preset.CropPX.X < 0 || preset.CropPX.Y < 0 || preset.CropPX.X+preset.CropPX.W > p.SourceWidth || preset.CropPX.Y+preset.CropPX.H > p.SourceHeight {
			return fmt.Errorf("preset %s crop is outside source bounds", preset.ID)
		}
	}
	return nil
}

func WritePresets(projectPath string, presets Presets) error {
	raw, err := json.MarshalIndent(presets, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(projectPath, "calibration", "framing-presets.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func ReadPresets(projectPath string) (Presets, error) {
	raw, err := os.ReadFile(filepath.Join(projectPath, "calibration", "framing-presets.json"))
	if err != nil {
		return Presets{}, err
	}
	return ParsePresets(raw)
}
