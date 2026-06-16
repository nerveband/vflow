package nle

import (
	"strings"
	"testing"
)

func TestParseImportDetectsFCPXMLRoundtripChanges(t *testing.T) {
	result, err := ParseImport("roundtrip.fcpxml", []byte(roundtripFCPXML))
	if err != nil {
		t.Fatalf("ParseImport returned error: %v", err)
	}
	if result.Version != "vflow-nle-import/v1" || result.Format != "fcpxml" || result.Status != "parsed" {
		t.Fatalf("unexpected import header: %+v", result)
	}
	for _, want := range []string{"clip_trim", "marker_note", "audio_level", "title_card", "crop_change", "color_grade"} {
		if !hasChangeType(result.Changes, want) {
			t.Fatalf("missing change type %q in %+v", want, result.Changes)
		}
	}
	if got := segmentForType(result.Changes, "clip_trim"); got != "seg_A" {
		t.Fatalf("expected clip trim to retain segment id seg_A, got %q", got)
	}
}

func TestParseImportsFromAllExportTargetsPreserveSegmentIdentity(t *testing.T) {
	segments := []Segment{{ID: "seg_A", SourceFrameIn: 12, SourceFrameOut: 60, TimelineFrameIn: 0, TimelineFrameOut: 48}}
	opts := Options{SourceMediaID: "camera_a", SourceURL: "file:///camera-a.mp4", Rate: 24}
	for _, target := range []string{"edl", "fcpxml", "resolve", "premiere", "mlt", "otio"} {
		t.Run(target, func(t *testing.T) {
			opts.Target = target
			input := "timeline." + target
			if target == "resolve" {
				input = "timeline.fcpxml"
			}
			result, err := ParseImport(input, []byte(exportText(Export(opts, segments))))
			if err != nil {
				t.Fatalf("ParseImport(%s) returned error: %v", target, err)
			}
			if !hasChangeType(result.Changes, "clip_trim") {
				t.Fatalf("%s import missing clip_trim: %+v", target, result.Changes)
			}
			if got := segmentForType(result.Changes, "clip_trim"); got != "seg_A" {
				t.Fatalf("%s import did not preserve segment id: got %q changes=%+v", target, got, result.Changes)
			}
			diff := Classify(result)
			if len(diff.Unclassified) != 0 || len(diff.SafeMerge) == 0 {
				t.Fatalf("%s diff should classify exported fixture cleanly: %+v", target, diff)
			}
		})
	}
}

func TestParseImportRejectsResolveProjectPackageWithActionableHint(t *testing.T) {
	_, err := ParseImport("Executive Directors.drp", []byte(`{"version":1,"mixEffectBlocks":[{"source":2}]}`))
	if err == nil || !strings.Contains(err.Error(), "export FCPXML, EDL, or OTIO from Resolve") {
		t.Fatalf("expected actionable .drp rejection, got %v", err)
	}
}

func TestParseEDLWithoutVflowSegmentIDIsBlockedAsMissingSidecar(t *testing.T) {
	result, err := ParseImport("editor-export.edl", []byte(`TITLE: editor export
FCM: NON-DROP FRAME
001  AX       V     C        00000012 00000060 00000000 00000048
`))
	if err != nil {
		t.Fatalf("ParseImport returned error: %v", err)
	}
	if got := segmentForType(result.Changes, "clip_trim"); got != "" {
		t.Fatalf("expected EDL clip trim without vflow segment id, got %q", got)
	}
	diff := Classify(result)
	if len(diff.SafeMerge) != 0 || !hasChangeType(diff.Blocked, "missing_sidecar") {
		t.Fatalf("expected missing sidecar ID to block roundtrip apply: %+v", diff)
	}
}

func TestFCPXMLMarkerValueWithoutVflowIDDoesNotBecomeSegmentID(t *testing.T) {
	result, err := ParseImport("editor-marker.fcpxml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<fcpxml version="1.11">
  <library><event name="Roundtrip"><project name="vflow"><sequence duration="48/24s"><spine>
    <asset-clip offset="0/24s" start="12/24s" duration="48/24s">
      <marker start="0s" value="producer note"/>
    </asset-clip>
  </spine></sequence></project></event></library>
</fcpxml>`))
	if err != nil {
		t.Fatalf("ParseImport returned error: %v", err)
	}
	if got := segmentForType(result.Changes, "marker_note"); got != "" {
		t.Fatalf("plain marker value should not be treated as segment id, got %q", got)
	}
	diff := Classify(result)
	if len(diff.SafeMerge) != 0 || !hasChangeType(diff.Blocked, "missing_sidecar") {
		t.Fatalf("expected marker without vflow identity to block roundtrip apply: %+v", diff)
	}
}

func TestFCPXMLMarkerValueWithVflowIDPreservesSegmentID(t *testing.T) {
	result, err := ParseImport("editor-marker.fcpxml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<fcpxml version="1.11">
  <library><event name="Roundtrip"><project name="vflow"><sequence duration="48/24s"><spine>
    <asset-clip offset="0/24s" start="12/24s" duration="48/24s">
      <marker start="0s" value="vflow:segment-id=seg_A"/>
    </asset-clip>
  </spine></sequence></project></event></library>
</fcpxml>`))
	if err != nil {
		t.Fatalf("ParseImport returned error: %v", err)
	}
	if got := segmentForType(result.Changes, "marker_note"); got != "seg_A" {
		t.Fatalf("expected vflow marker value to preserve segment id, got %q", got)
	}
}

func hasChangeType(changes []Change, typ string) bool {
	for _, change := range changes {
		if change.Type == typ {
			return true
		}
	}
	return false
}

func segmentForType(changes []Change, typ string) string {
	for _, change := range changes {
		if change.Type == typ {
			return change.SegmentID
		}
	}
	return ""
}

const roundtripFCPXML = `<?xml version="1.0" encoding="UTF-8"?>
<fcpxml version="1.11">
  <library>
    <event name="Roundtrip">
      <project name="vflow">
        <sequence duration="120/24s">
          <spine>
            <asset-clip name="seg_A" offset="0/24s" start="12/24s" duration="48/24s">
              <marker start="0s" value="producer note" note="vflow:segment-id=seg_A reviewed=yes"/>
              <adjust-volume amount="-3dB"/>
              <adjust-transform position="12 4" scale="1.05 1.05"/>
              <filter-video name="Lumetri Color"/>
            </asset-clip>
            <title name="Lower Third" offset="24/24s" duration="24/24s">
              <text><text-style ref="ts1">Executive Director</text-style></text>
            </title>
          </spine>
        </sequence>
      </project>
    </event>
  </library>
</fcpxml>`
