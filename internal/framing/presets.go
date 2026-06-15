package framing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	if p.SourceWidth <= 0 || p.SourceHeight <= 0 {
		return fmt.Errorf("source dimensions must be positive")
	}
	if p.TargetAspect == "" {
		return fmt.Errorf("target_aspect is required")
	}
	if len(p.Presets) == 0 {
		return fmt.Errorf("at least one preset is required")
	}
	ids := map[string]bool{}
	for _, preset := range p.Presets {
		if preset.ID == "" {
			return fmt.Errorf("preset id is required")
		}
		if looksLikeDiarizationLabel(preset.ID) || looksLikeDiarizationLabel(preset.Label) {
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

func (p Presets) IDSet() map[string]bool {
	ids := map[string]bool{}
	for _, preset := range p.Presets {
		ids[preset.ID] = true
	}
	return ids
}

var diarizationLabelPattern = regexp.MustCompile(`\b(?:SPEAKER[_-]?[A-Za-z0-9]+|speaker[_-]?\d+)\b`)

func looksLikeDiarizationLabel(value string) bool {
	return diarizationLabelPattern.MatchString(value)
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
