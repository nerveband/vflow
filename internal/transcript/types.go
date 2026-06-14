package transcript

type Word struct {
	ID           string  `json:"id"`
	Text         string  `json:"text"`
	SpeakerLabel string  `json:"speaker_label,omitempty"`
	StartFrame   int64   `json:"start_frame"`
	EndFrame     int64   `json:"end_frame"`
	Confidence   float64 `json:"confidence,omitempty"`
	Provider     string  `json:"provider"`
}

type Words struct {
	Version       string `json:"version"`
	SourceMediaID string `json:"source_media_id"`
	Rate          string `json:"rate"`
	Words         []Word `json:"words"`
}

type ImportOptions struct {
	SourceMediaID string
	Rate          string
	FramesPerWord int64
}
