package nle

type Segment struct {
	ID               string   `json:"id"`
	VflowSegmentID   string   `json:"vflow_segment_id,omitempty"`
	SourceMediaID    string   `json:"source_media_id,omitempty"`
	SourceFrameIn    int      `json:"source_frame_in"`
	SourceFrameOut   int      `json:"source_frame_out"`
	TimelineFrameIn  int      `json:"timeline_frame_in"`
	TimelineFrameOut int      `json:"timeline_frame_out"`
	ContentEDLID     string   `json:"content_edl_id,omitempty"`
	FramingPresetID  string   `json:"framing_preset_id,omitempty"`
	MarkerIDs        []string `json:"marker_ids,omitempty"`
	ExportTarget     string   `json:"export_target,omitempty"`
	ExportVersion    string   `json:"export_version,omitempty"`
}

type Options struct {
	Target        string `json:"target"`
	Output        string `json:"output"`
	SourceMediaID string `json:"source_media_id,omitempty"`
	SourceURL     string `json:"source_url,omitempty"`
	Rate          int    `json:"rate,omitempty"`
	ProjectName   string `json:"project_name,omitempty"`
}

type Sidecar struct {
	Version  string    `json:"version"`
	Target   string    `json:"target"`
	Segments []Segment `json:"segments"`
}

type ExportResult struct {
	Target  string  `json:"target"`
	Output  string  `json:"output"`
	Options Options `json:"options"`
	Sidecar Sidecar `json:"sidecar"`
}

type Change struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	SegmentID   string  `json:"segment_id,omitempty"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

type ImportResult struct {
	Version  string   `json:"version"`
	Status   string   `json:"status"`
	Input    string   `json:"input"`
	Format   string   `json:"format"`
	Bytes    int      `json:"bytes"`
	Artifact string   `json:"artifact,omitempty"`
	Changes  []Change `json:"changes"`
}

type DiffResult struct {
	Version      string   `json:"version"`
	Status       string   `json:"status"`
	Import       string   `json:"import"`
	Format       string   `json:"format"`
	Artifact     string   `json:"artifact,omitempty"`
	SafeMerge    []Change `json:"safe_merge"`
	NeedsReview  []Change `json:"needs_review"`
	Blocked      []Change `json:"blocked"`
	Unclassified []Change `json:"unclassified"`
}

type ApplyPlan struct {
	Version     string   `json:"version"`
	Status      string   `json:"status"`
	Artifact    string   `json:"artifact,omitempty"`
	Applied     []Change `json:"applied"`
	NeedsReview []Change `json:"needs_review"`
	Blocked     []Change `json:"blocked"`
}
