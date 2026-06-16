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
