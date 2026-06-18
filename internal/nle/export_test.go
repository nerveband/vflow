package nle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExportSidecarMapsSourceAndTimelineFrames(t *testing.T) {
	result := Export(Options{Target: "edl", Output: "timeline.edl"}, []Segment{{ID: "seg_A", SourceFrameIn: 0, SourceFrameOut: 90, TimelineFrameIn: 0, TimelineFrameOut: 90}})
	if len(result.Sidecar.Segments) != 1 {
		t.Fatalf("expected one sidecar segment")
	}
	if result.Sidecar.Segments[0].SourceFrameOut != 90 {
		t.Fatalf("unexpected source frame out: %d", result.Sidecar.Segments[0].SourceFrameOut)
	}
	if result.Target != "edl" {
		t.Fatalf("unexpected target: %s", result.Target)
	}
}

func TestExportSidecarIncludesRequiredRoundtripMetadata(t *testing.T) {
	result := Export(Options{Target: "fcpxml", Output: "timeline.fcpxml", SourceMediaID: "camera_a", SourceURL: "file:///camera-a.mp4", Rate: 24}, []Segment{{ID: "seg_A", SourceFrameIn: 12, SourceFrameOut: 60, TimelineFrameIn: 0, TimelineFrameOut: 48}})
	segment := result.Sidecar.Segments[0]
	if segment.VflowSegmentID != "seg_A" || segment.SourceMediaID != "camera_a" || segment.ExportTarget != "fcpxml" || segment.ExportVersion == "" {
		t.Fatalf("missing sidecar metadata: %+v", segment)
	}
	if len(segment.MarkerIDs) == 0 {
		t.Fatalf("expected marker ids for roundtrip identity: %+v", segment)
	}
}

func TestValidTargetRejectsMistypedNLETargets(t *testing.T) {
	for _, target := range []string{"edl", "fcpxml", "resolve", "premiere", "mlt", "otio", "sidecar"} {
		if !ValidTarget(target) {
			t.Fatalf("expected %q to be valid", target)
		}
	}
	for _, target := range []string{"", "fcp", "fcpxm", "premiere-xml", "xml"} {
		if ValidTarget(target) {
			t.Fatalf("expected %q to be invalid", target)
		}
	}
}

func TestExportTextsContainInterchangeSegments(t *testing.T) {
	segments := []Segment{{ID: "seg_A", SourceFrameIn: 12, SourceFrameOut: 60, TimelineFrameIn: 0, TimelineFrameOut: 48}}
	opts := Options{SourceMediaID: "camera_a", SourceURL: "file:///camera-a.mp4", Rate: 24}
	for _, tc := range []struct {
		target string
		wants  []string
	}{
		{"edl", []string{"TITLE: vflow", "001", "AX", "00000012", "00000060"}},
		{"fcpxml", []string{"<fcpxml", "<asset-clip", "vflow:segment-id", "seg_A"}},
		{"premiere", []string{"<xmeml", "<clipitem", "<name>seg_A</name>", "<pathurl>file:///camera-a.mp4</pathurl>"}},
		{"mlt", []string{"<mlt", "<producer", "<playlist", "seg_A"}},
		{"resolve", []string{"<fcpxml", "resolve", "seg_A"}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			opts.Target = tc.target
			text := exportText(Export(opts, segments))
			for _, want := range tc.wants {
				if !strings.Contains(text, want) {
					t.Fatalf("%s export missing %q in:\n%s", tc.target, want, text)
				}
			}
		})
	}
}

func TestOTIOExportIsStructuredTimelineJSON(t *testing.T) {
	result := Export(Options{Target: "otio", SourceMediaID: "camera_a", SourceURL: "file:///camera-a.mp4", Rate: 24}, []Segment{{ID: "seg_A", SourceFrameIn: 12, SourceFrameOut: 60, TimelineFrameIn: 0, TimelineFrameOut: 48}})
	var got map[string]any
	if err := json.Unmarshal([]byte(exportText(result)), &got); err != nil {
		t.Fatalf("invalid otio json: %v", err)
	}
	if got["OTIO_SCHEMA"] != "Timeline.1" {
		t.Fatalf("unexpected OTIO schema: %+v", got)
	}
	tracks := got["tracks"].(map[string]any)
	children := tracks["children"].([]any)
	track := children[0].(map[string]any)
	clips := track["children"].([]any)
	clip := clips[0].(map[string]any)
	if clip["OTIO_SCHEMA"] != "Clip.1" || clip["name"] != "seg_A" {
		t.Fatalf("unexpected OTIO clip: %+v", clip)
	}
}

func TestOTIOExportPreservesVideoAndAudioTrackMetadata(t *testing.T) {
	result := Export(Options{Target: "otio", SourceMediaID: "camera_a", SourceURL: "file:///camera-a.mp4", Rate: 24}, []Segment{
		{ID: "seg_A_v", TrackID: "v1", TrackKind: "video", LinkedClipID: "seg_A_a", SourceFrameIn: 12, SourceFrameOut: 60, TimelineFrameIn: 0, TimelineFrameOut: 48},
		{ID: "seg_A_a", TrackID: "a1", TrackKind: "audio", LinkedClipID: "seg_A_v", SourceFrameIn: 12, SourceFrameOut: 60, TimelineFrameIn: 0, TimelineFrameOut: 48},
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(exportText(result)), &got); err != nil {
		t.Fatalf("invalid otio json: %v", err)
	}
	children := got["tracks"].(map[string]any)["children"].([]any)
	if len(children) != 2 {
		t.Fatalf("expected separate video/audio OTIO tracks, got %+v", children)
	}
	audioTrack := children[1].(map[string]any)
	if audioTrack["kind"] != "Audio" {
		t.Fatalf("expected audio track kind, got %+v", audioTrack)
	}
	audioClip := audioTrack["children"].([]any)[0].(map[string]any)
	vflowMeta := audioClip["metadata"].(map[string]any)["vflow"].(map[string]any)
	if vflowMeta["linked_clip_id"] != "seg_A_v" || vflowMeta["track_id"] != "a1" {
		t.Fatalf("expected linked clip metadata in OTIO, got %+v", vflowMeta)
	}
}

func TestVerifySidecarBlocksCoverageAndDriftProblems(t *testing.T) {
	report := Verify(Sidecar{
		Version: "vflow-nle-sidecar/v1",
		Target:  "otio",
		Segments: []Segment{
			{ID: "seg_A", VflowSegmentID: "seg_A", SourceMediaID: "camera_a", SourceFrameIn: 0, SourceFrameOut: 30, TimelineFrameIn: 0, TimelineFrameOut: 30, MarkerIDs: []string{"m1"}, ExportTarget: "otio", ExportVersion: exportVersion},
			{ID: "seg_B", VflowSegmentID: "", SourceMediaID: "camera_a", SourceFrameIn: 30, SourceFrameOut: 60, TimelineFrameIn: 29, TimelineFrameOut: 60, MarkerIDs: nil, ExportTarget: "otio", ExportVersion: exportVersion},
		},
	}, []Segment{{ID: "seg_A", SourceFrameIn: 0, SourceFrameOut: 30, TimelineFrameIn: 0, TimelineFrameOut: 30}, {ID: "seg_B", SourceFrameIn: 30, SourceFrameOut: 60, TimelineFrameIn: 30, TimelineFrameOut: 60}})

	if report.Status != "blocked" {
		t.Fatalf("expected blocked sidecar verification, got %+v", report)
	}
	for _, want := range []string{"MISSING_SIDECAR_ID", "MISSING_MARKER_ID", "TRIM_DRIFT"} {
		if !hasIssueCode(report.Issues, want) {
			t.Fatalf("missing issue %s in %+v", want, report.Issues)
		}
	}
}

func hasIssueCode(issues []VerifyIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
