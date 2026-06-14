package timeline

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/nerveband/vflow/internal/cleanup"
)

type CompiledTimeline struct {
	Version        string       `json:"version"`
	DurationFrames int          `json:"duration_frames"`
	TimeMap        Map          `json:"time_map"`
	Segments       []Segment    `json:"segments"`
	ReviewItems    []ReviewItem `json:"review_items"`
}

type Segment struct {
	ID               string `json:"id"`
	SourceFrameIn    int    `json:"source_frame_in"`
	SourceFrameOut   int    `json:"source_frame_out"`
	TimelineFrameIn  int    `json:"timeline_frame_in"`
	TimelineFrameOut int    `json:"timeline_frame_out"`
}

type ReviewItem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Compile(edl cleanup.ContentEDL, durationFrames int) CompiledTimeline {
	deletes := make([]DeleteRange, 0, len(edl.DeleteSegments))
	for _, del := range edl.DeleteSegments {
		deletes = append(deletes, DeleteRange{StartFrame: del.StartFrame, EndFrame: del.EndFrame})
	}
	m := BuildTimeMap(deletes, durationFrames)
	tl := CompiledTimeline{Version: "vflow-compiled-timeline/v1", DurationFrames: durationFrames, TimeMap: m}
	lastSource := 0
	for i, del := range deletes {
		if del.StartFrame > lastSource {
			in, _ := m.SourceToTimeline(lastSource)
			out, _ := m.SourceToTimeline(del.StartFrame)
			tl.Segments = append(tl.Segments, Segment{ID: segmentID(len(tl.Segments) + 1), SourceFrameIn: lastSource, SourceFrameOut: del.StartFrame, TimelineFrameIn: in, TimelineFrameOut: out})
		}
		lastSource = del.EndFrame
		_ = i
	}
	if durationFrames > lastSource {
		in, _ := m.SourceToTimeline(lastSource)
		out, _ := m.SourceToTimeline(durationFrames)
		tl.Segments = append(tl.Segments, Segment{ID: segmentID(len(tl.Segments) + 1), SourceFrameIn: lastSource, SourceFrameOut: durationFrames, TimelineFrameIn: in, TimelineFrameOut: out})
	}
	return tl
}

func WriteCompiled(projectPath string, tl CompiledTimeline) error {
	raw, err := json.MarshalIndent(tl, "", "  ")
	if err != nil {
		return err
	}
	timeMapRaw, err := json.MarshalIndent(tl.TimeMap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(projectPath, "timeline"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(projectPath, "decisions"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectPath, "decisions", "time-map.json"), append(timeMapRaw, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectPath, "timeline", "compiled-timeline.json"), append(raw, '\n'), 0o644)
}

func segmentID(n int) string {
	return "seg_" + string(rune('A'+n-1))
}
