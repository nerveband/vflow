package nle

type Segment struct {
	ID               string `json:"id"`
	SourceFrameIn    int    `json:"source_frame_in"`
	SourceFrameOut   int    `json:"source_frame_out"`
	TimelineFrameIn  int    `json:"timeline_frame_in"`
	TimelineFrameOut int    `json:"timeline_frame_out"`
}

type Options struct {
	Target string `json:"target"`
	Output string `json:"output"`
}

type Sidecar struct {
	Version  string    `json:"version"`
	Target   string    `json:"target"`
	Segments []Segment `json:"segments"`
}

type ExportResult struct {
	Target  string  `json:"target"`
	Output  string  `json:"output"`
	Sidecar Sidecar `json:"sidecar"`
}
