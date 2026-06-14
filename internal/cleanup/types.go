package cleanup

type DeleteSegment struct {
	ID          string  `json:"id"`
	StartFrame  int     `json:"start_frame"`
	EndFrame    int     `json:"end_frame"`
	Reason      string  `json:"reason"`
	Confidence  float64 `json:"confidence"`
	SourceStart float64 `json:"source_start_seconds,omitempty"`
	SourceEnd   float64 `json:"source_end_seconds,omitempty"`
}

type ContentEDL struct {
	Version        string          `json:"version"`
	Rate           string          `json:"rate"`
	DeleteSegments []DeleteSegment `json:"delete_segments"`
}
