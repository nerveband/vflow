package nle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const exportVersion = "vflow-nle-export/v1"

func Export(opts Options, segments []Segment) ExportResult {
	if opts.Target == "" {
		opts.Target = "sidecar"
	}
	if opts.Rate <= 0 {
		opts.Rate = 30
	}
	if opts.SourceMediaID == "" {
		opts.SourceMediaID = "source"
	}
	if opts.SourceURL == "" {
		opts.SourceURL = "file://media/source.mp4"
	}
	if opts.ProjectName == "" {
		opts.ProjectName = "vflow"
	}
	normalized := make([]Segment, 0, len(segments))
	for i, segment := range segments {
		if segment.ID == "" {
			segment.ID = fmt.Sprintf("seg_%06d", i+1)
		}
		if segment.VflowSegmentID == "" {
			segment.VflowSegmentID = segment.ID
		}
		if segment.SourceMediaID == "" {
			segment.SourceMediaID = opts.SourceMediaID
		}
		if len(segment.MarkerIDs) == 0 {
			segment.MarkerIDs = []string{"vflow_marker_" + segment.VflowSegmentID}
		}
		segment.ExportTarget = opts.Target
		segment.ExportVersion = exportVersion
		normalized = append(normalized, segment)
	}
	return ExportResult{
		Target: opts.Target,
		Output: opts.Output,
		Options: Options{
			Target:        opts.Target,
			Output:        opts.Output,
			SourceMediaID: opts.SourceMediaID,
			SourceURL:     opts.SourceURL,
			Rate:          opts.Rate,
			ProjectName:   opts.ProjectName,
		},
		Sidecar: Sidecar{
			Version:  "vflow-nle-sidecar/v1",
			Target:   opts.Target,
			Segments: normalized,
		},
	}
}

func WriteExport(projectPath string, res ExportResult) error {
	if res.Output != "" {
		if err := os.MkdirAll(filepath.Dir(res.Output), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(res.Output, []byte(exportText(res)+"\n"), 0o644); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(res.Sidecar, "", "  ")
	if err != nil {
		return err
	}
	sidecarPath := filepath.Join(projectPath, "exports", "sidecars", res.Target+"-vflow-sidecar.json")
	if err := os.MkdirAll(filepath.Dir(sidecarPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(sidecarPath, append(raw, '\n'), 0o644)
}

func exportText(res ExportResult) string {
	opts := res.Options
	if opts.Target == "" {
		opts.Target = res.Target
	}
	if opts.Rate <= 0 {
		opts.Rate = 30
	}
	if opts.SourceMediaID == "" {
		opts.SourceMediaID = "source"
	}
	if opts.SourceURL == "" {
		opts.SourceURL = "file://media/source.mp4"
	}
	if opts.ProjectName == "" {
		opts.ProjectName = "vflow"
	}
	switch res.Target {
	case "edl":
		return exportEDL(opts, res.Sidecar.Segments)
	case "fcpxml":
		return exportFCPXML(opts, res.Sidecar.Segments, "fcpxml")
	case "resolve":
		return exportFCPXML(opts, res.Sidecar.Segments, "resolve")
	case "premiere":
		return exportPremiereXML(opts, res.Sidecar.Segments)
	case "mlt":
		return exportMLT(opts, res.Sidecar.Segments)
	case "otio":
		return exportOTIO(opts, res.Sidecar.Segments)
	default:
		raw, _ := json.MarshalIndent(res.Sidecar, "", "  ")
		return string(raw)
	}
}

func exportEDL(opts Options, segments []Segment) string {
	var b strings.Builder
	b.WriteString("TITLE: vflow\nFCM: NON-DROP FRAME\n")
	for i, segment := range segments {
		event := fmt.Sprintf("%03d", i+1)
		b.WriteString(fmt.Sprintf("%s  AX       V     C        %s %s %s %s\n",
			event,
			frameCode(segment.SourceFrameIn),
			frameCode(segment.SourceFrameOut),
			frameCode(segment.TimelineFrameIn),
			frameCode(segment.TimelineFrameOut),
		))
		b.WriteString(fmt.Sprintf("* FROM CLIP NAME: %s\n", segment.VflowSegmentID))
		b.WriteString(fmt.Sprintf("* VFLOW-SEGMENT-ID: %s\n", segment.VflowSegmentID))
	}
	return strings.TrimRight(b.String(), "\n")
}

func exportFCPXML(opts Options, segments []Segment, flavor string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<fcpxml version="1.11">` + "\n")
	b.WriteString(`  <resources>` + "\n")
	b.WriteString(fmt.Sprintf(`    <format id="r1" name="FFVideoFormat1080p%d" frameDuration="1/%ds" width="1920" height="1080"/>`, opts.Rate, opts.Rate) + "\n")
	b.WriteString(fmt.Sprintf(`    <asset id="r2" name="%s" src="%s" start="0s" hasVideo="1" hasAudio="1"/>`, xmlEsc(opts.SourceMediaID), xmlEsc(opts.SourceURL)) + "\n")
	b.WriteString(`  </resources>` + "\n")
	b.WriteString(`  <library><event name="vflow"><project name="vflow">` + "\n")
	b.WriteString(fmt.Sprintf(`    <sequence format="r1" duration="%d/%ds" tcStart="0s" tcFormat="NDF">`, totalTimelineFrames(segments), opts.Rate) + "\n")
	b.WriteString(`      <spine>` + "\n")
	for _, segment := range segments {
		b.WriteString(fmt.Sprintf(`        <asset-clip ref="r2" name="%s" offset="%d/%ds" start="%d/%ds" duration="%d/%ds">`,
			xmlEsc(segment.VflowSegmentID),
			segment.TimelineFrameIn, opts.Rate,
			segment.SourceFrameIn, opts.Rate,
			segment.SourceFrameOut-segment.SourceFrameIn, opts.Rate,
		) + "\n")
		b.WriteString(fmt.Sprintf(`          <marker start="0s" value="%s" note="vflow:segment-id=%s target=%s"/>`,
			xmlEsc(firstMarker(segment)),
			xmlEsc(segment.VflowSegmentID),
			xmlEsc(flavor),
		) + "\n")
		b.WriteString(`        </asset-clip>` + "\n")
	}
	b.WriteString(`      </spine>` + "\n")
	b.WriteString(`    </sequence>` + "\n")
	b.WriteString(`  </project></event></library>` + "\n")
	b.WriteString(`</fcpxml>`)
	return b.String()
}

func exportPremiereXML(opts Options, segments []Segment) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<xmeml version="5"><sequence id="vflow-sequence"><name>vflow</name><rate><timebase>`)
	b.WriteString(fmt.Sprintf("%d", opts.Rate))
	b.WriteString(`</timebase><ntsc>FALSE</ntsc></rate><media><video><track>` + "\n")
	for i, segment := range segments {
		b.WriteString(fmt.Sprintf(`<clipitem id="clipitem-%d"><name>%s</name><start>%d</start><end>%d</end><in>%d</in><out>%d</out><file id="file-%d"><name>%s</name><pathurl>%s</pathurl></file><comments>vflow:segment-id=%s</comments></clipitem>`+"\n",
			i+1,
			xmlEsc(segment.VflowSegmentID),
			segment.TimelineFrameIn,
			segment.TimelineFrameOut,
			segment.SourceFrameIn,
			segment.SourceFrameOut,
			i+1,
			xmlEsc(opts.SourceMediaID),
			xmlEsc(opts.SourceURL),
			xmlEsc(segment.VflowSegmentID),
		))
	}
	b.WriteString(`</track></video></media></sequence></xmeml>`)
	return b.String()
}

func exportMLT(opts Options, segments []Segment) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<mlt title="vflow">` + "\n")
	b.WriteString(fmt.Sprintf(`  <producer id="producer0"><property name="resource">%s</property></producer>`, xmlEsc(opts.SourceURL)) + "\n")
	b.WriteString(`  <playlist id="playlist0">` + "\n")
	for _, segment := range segments {
		b.WriteString(fmt.Sprintf(`    <entry producer="producer0" in="%d" out="%d"><property name="vflow:segment-id">%s</property></entry>`,
			segment.SourceFrameIn,
			segment.SourceFrameOut-1,
			xmlEsc(segment.VflowSegmentID),
		) + "\n")
	}
	b.WriteString(`  </playlist>` + "\n")
	b.WriteString(`  <tractor id="tractor0"><track producer="playlist0"/></tractor>` + "\n")
	b.WriteString(`</mlt>`)
	return b.String()
}

func exportOTIO(opts Options, segments []Segment) string {
	clips := make([]map[string]any, 0, len(segments))
	for _, segment := range segments {
		clips = append(clips, map[string]any{
			"OTIO_SCHEMA": "Clip.1",
			"name":        segment.VflowSegmentID,
			"effects":     []any{},
			"markers": []map[string]any{{
				"OTIO_SCHEMA": "Marker.2",
				"name":        firstMarker(segment),
				"metadata":    map[string]any{"vflow": map[string]any{"segment_id": segment.VflowSegmentID}},
			}},
			"metadata": map[string]any{"vflow": segment},
			"media_reference": map[string]any{
				"OTIO_SCHEMA": "ExternalReference.1",
				"target_url":  opts.SourceURL,
			},
			"source_range": timeRange(segment.SourceFrameIn, segment.SourceFrameOut-segment.SourceFrameIn, opts.Rate),
		})
	}
	raw, _ := json.MarshalIndent(map[string]any{
		"OTIO_SCHEMA": "Timeline.1",
		"name":        "vflow",
		"metadata":    map[string]any{"vflow": map[string]any{"export_version": exportVersion}},
		"tracks": map[string]any{
			"OTIO_SCHEMA":  "Stack.1",
			"name":         "tracks",
			"effects":      []any{},
			"markers":      []any{},
			"metadata":     map[string]any{},
			"source_range": nil,
			"children": []map[string]any{{
				"OTIO_SCHEMA":  "Track.1",
				"name":         "Video 1",
				"kind":         "Video",
				"effects":      []any{},
				"markers":      []any{},
				"metadata":     map[string]any{},
				"source_range": nil,
				"children":     clips,
			}},
		},
	}, "", "  ")
	return string(raw)
}

func timeRange(start, duration, rate int) map[string]any {
	return map[string]any{
		"OTIO_SCHEMA": "TimeRange.1",
		"start_time": map[string]any{
			"OTIO_SCHEMA": "RationalTime.1",
			"value":       start,
			"rate":        rate,
		},
		"duration": map[string]any{
			"OTIO_SCHEMA": "RationalTime.1",
			"value":       duration,
			"rate":        rate,
		},
	}
}

func totalTimelineFrames(segments []Segment) int {
	total := 0
	for _, segment := range segments {
		if segment.TimelineFrameOut > total {
			total = segment.TimelineFrameOut
		}
	}
	return total
}

func frameCode(frame int) string {
	return fmt.Sprintf("%08d", frame)
}

func firstMarker(segment Segment) string {
	if len(segment.MarkerIDs) > 0 {
		return segment.MarkerIDs[0]
	}
	return "vflow_marker_" + segment.VflowSegmentID
}

func xmlEsc(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
}
