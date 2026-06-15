package framing

type Policy struct {
	Version                string  `json:"version"`
	MinDwellFrames         int64   `json:"min_dwell_frames"`
	LowConfidenceThreshold float64 `json:"low_confidence_threshold"`
	WidePresetID           string  `json:"wide_preset_id"`
	WideResetFrames        int64   `json:"wide_reset_frames"`
}

func DefaultPolicy() Policy {
	return Policy{
		Version:                "vflow-framing-policy/v1",
		MinDwellFrames:         45,
		LowConfidenceThreshold: 0.70,
		WidePresetID:           "wide",
		WideResetFrames:        90,
	}
}

func (p Policy) withDefaults() Policy {
	defaults := DefaultPolicy()
	if p.Version == "" {
		p.Version = defaults.Version
	}
	if p.MinDwellFrames <= 0 {
		p.MinDwellFrames = defaults.MinDwellFrames
	}
	if p.LowConfidenceThreshold <= 0 {
		p.LowConfidenceThreshold = defaults.LowConfidenceThreshold
	}
	if p.WidePresetID == "" {
		p.WidePresetID = defaults.WidePresetID
	}
	if p.WideResetFrames <= 0 {
		p.WideResetFrames = defaults.WideResetFrames
	}
	return p
}
