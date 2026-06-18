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
	Canonical      Timeline     `json:"canonical"`
}

type Segment struct {
	ID               string          `json:"id"`
	SourceFrameIn    int             `json:"source_frame_in"`
	SourceFrameOut   int             `json:"source_frame_out"`
	TimelineFrameIn  int             `json:"timeline_frame_in"`
	TimelineFrameOut int             `json:"timeline_frame_out"`
	Provenance       FrameProvenance `json:"provenance"`
}

type Timeline struct {
	Version           string             `json:"version"`
	FPS               string             `json:"fps"`
	DurationFrames    int                `json:"duration_frames"`
	Tracks            []Track            `json:"tracks"`
	SyncMapRefs       []string           `json:"sync_map_refs,omitempty"`
	TranscriptAnchors []TranscriptAnchor `json:"transcript_anchors,omitempty"`
	MulticamGroups    []MulticamGroup    `json:"multicam_groups,omitempty"`
	ActiveAngleSpans  []ActiveAngleSpan  `json:"active_angle_spans,omitempty"`
}

type Track struct {
	ID        string `json:"id"`
	TrackType string `json:"track_type"`
	Name      string `json:"name,omitempty"`
	Clips     []Clip `json:"clips"`
}

type Clip struct {
	ID               string     `json:"id"`
	StableClipID     string     `json:"stable_clip_id"`
	TrackID          string     `json:"track_id"`
	TrackType        string     `json:"track_type"`
	SourceMediaID    string     `json:"source_media_id,omitempty"`
	LinkedClipID     string     `json:"linked_clip_id,omitempty"`
	SyncMapRef       string     `json:"sync_map_ref,omitempty"`
	TranscriptAnchor string     `json:"transcript_anchor,omitempty"`
	MulticamGroupID  string     `json:"multicam_group_id,omitempty"`
	AngleID          string     `json:"angle_id,omitempty"`
	SourceRange      FrameRange `json:"source_range"`
	TimelineRange    FrameRange `json:"timeline_range"`
}

type FrameRange struct {
	StartFrame int `json:"start_frame"`
	EndFrame   int `json:"end_frame"`
}

type TranscriptAnchor struct {
	ID                 string `json:"id"`
	TranscriptFrameIn  int    `json:"transcript_frame_in"`
	TranscriptFrameOut int    `json:"transcript_frame_out"`
	ClipID             string `json:"clip_id"`
}

type MulticamGroup struct {
	ID             string   `json:"id"`
	SyncMapRef     string   `json:"sync_map_ref"`
	ReferenceAngle string   `json:"reference_angle"`
	AngleIDs       []string `json:"angle_ids"`
}

type ActiveAngleSpan struct {
	ID              string     `json:"id"`
	MulticamGroupID string     `json:"multicam_group_id"`
	AngleID         string     `json:"angle_id"`
	TimelineRange   FrameRange `json:"timeline_range"`
}

type FrameProvenance struct {
	SourceFrameIn    int `json:"source_frame_in"`
	SourceFrameOut   int `json:"source_frame_out"`
	TimelineFrameIn  int `json:"timeline_frame_in"`
	TimelineFrameOut int `json:"timeline_frame_out"`
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
			in := m.SourceBoundaryToTimeline(lastSource)
			out := m.SourceBoundaryToTimeline(del.StartFrame)
			tl.Segments = append(tl.Segments, newSegment(len(tl.Segments)+1, lastSource, del.StartFrame, in, out))
		}
		lastSource = del.EndFrame
		_ = i
	}
	if durationFrames > lastSource {
		in := m.SourceBoundaryToTimeline(lastSource)
		out := m.SourceBoundaryToTimeline(durationFrames)
		tl.Segments = append(tl.Segments, newSegment(len(tl.Segments)+1, lastSource, durationFrames, in, out))
	}
	tl.Canonical = BuildCanonical(tl.Segments, durationFrames, "30/1")
	return tl
}

func BuildCanonical(segments []Segment, durationFrames int, fps string) Timeline {
	if fps == "" {
		fps = "30/1"
	}
	video := Track{ID: "v1", TrackType: "video", Name: "Video 1"}
	audio := Track{ID: "a1", TrackType: "audio", Name: "Audio 1"}
	for _, segment := range segments {
		videoID := segment.ID + "_v"
		audioID := segment.ID + "_a"
		ranges := clipRanges(segment)
		video.Clips = append(video.Clips, Clip{
			ID:            videoID,
			StableClipID:  segment.ID,
			TrackID:       video.ID,
			TrackType:     video.TrackType,
			SourceMediaID: "source",
			LinkedClipID:  audioID,
			SourceRange:   ranges.Source,
			TimelineRange: ranges.Timeline,
		})
		audio.Clips = append(audio.Clips, Clip{
			ID:            audioID,
			StableClipID:  segment.ID,
			TrackID:       audio.ID,
			TrackType:     audio.TrackType,
			SourceMediaID: "source",
			LinkedClipID:  videoID,
			SourceRange:   ranges.Source,
			TimelineRange: ranges.Timeline,
		})
	}
	return Timeline{
		Version:        "vflow-timeline/v1",
		FPS:            fps,
		DurationFrames: durationFrames,
		Tracks:         []Track{video, audio},
	}
}

type clipRangePair struct {
	Source   FrameRange
	Timeline FrameRange
}

func clipRanges(segment Segment) clipRangePair {
	return clipRangePair{
		Source:   FrameRange{StartFrame: segment.SourceFrameIn, EndFrame: segment.SourceFrameOut},
		Timeline: FrameRange{StartFrame: segment.TimelineFrameIn, EndFrame: segment.TimelineFrameOut},
	}
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
	if err := os.WriteFile(filepath.Join(projectPath, "timeline", "compiled-timeline.json"), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	canonicalRaw, err := json.MarshalIndent(tl.Canonical, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectPath, "timeline", "vflow-timeline.json"), append(canonicalRaw, '\n'), 0o644)
}

func segmentID(n int) string {
	return "seg_" + string(rune('A'+n-1))
}

func newSegment(n, sourceIn, sourceOut, timelineIn, timelineOut int) Segment {
	return Segment{
		ID:               segmentID(n),
		SourceFrameIn:    sourceIn,
		SourceFrameOut:   sourceOut,
		TimelineFrameIn:  timelineIn,
		TimelineFrameOut: timelineOut,
		Provenance: FrameProvenance{
			SourceFrameIn:    sourceIn,
			SourceFrameOut:   sourceOut,
			TimelineFrameIn:  timelineIn,
			TimelineFrameOut: timelineOut,
		},
	}
}
