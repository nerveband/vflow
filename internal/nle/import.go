package nle

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func ParseImport(input string, raw []byte) (ImportResult, error) {
	result := ImportResult{
		Version: "vflow-nle-import/v1",
		Status:  "parsed",
		Input:   filepath.ToSlash(input),
		Format:  detectFormat(input, raw),
		Bytes:   len(raw),
		Changes: []Change{},
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return result, errors.New("empty NLE import")
	}

	var (
		changes []Change
		err     error
	)
	switch result.Format {
	case "resolve-project":
		return result, fmt.Errorf("DaVinci Resolve .drp project packages are not timeline interchange files; export FCPXML, EDL, or OTIO from Resolve for vflow roundtrip import")
	case "fcpxml", "premiere", "mlt", "resolve":
		changes, err = parseXMLChanges(raw)
	case "otio":
		changes, err = parseOTIOChanges(raw)
	case "edl":
		changes = parseEDLChanges(raw)
	default:
		return result, fmt.Errorf("unsupported NLE import format %q", result.Format)
	}
	if err != nil {
		return result, err
	}
	result.Changes = assignChangeIDs(changes)
	return result, nil
}

func detectFormat(input string, raw []byte) string {
	lowerInput := strings.ToLower(input)
	lowerRaw := strings.ToLower(string(raw))
	switch {
	case strings.Contains(lowerRaw, "<fcpxml"):
		return "fcpxml"
	case strings.Contains(lowerRaw, "<xmeml"):
		return "premiere"
	case strings.Contains(lowerRaw, "<mlt"):
		return "mlt"
	case strings.Contains(lowerRaw, `"otio_schema"`) || strings.Contains(lowerRaw, "opentimelineio"):
		return "otio"
	case strings.Contains(lowerRaw, "title:") && strings.Contains(lowerRaw, "fcm:"):
		return "edl"
	}

	switch filepath.Ext(lowerInput) {
	case ".drp", ".dra", ".drt":
		return "resolve-project"
	case ".fcpxml":
		return "fcpxml"
	case ".xml":
		return "premiere"
	case ".mlt":
		return "mlt"
	case ".otio":
		return "otio"
	case ".edl":
		return "edl"
	default:
		return "unknown"
	}
}

func parseXMLChanges(raw []byte) ([]Change, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	changes := []Change{}
	currentSegment := ""
	pendingTiming := false
	captureSegmentText := ""

	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			attrs := xmlAttrs(t)
			switch name {
			case "asset-clip":
				currentSegment = firstNonEmpty(attrs["name"], attrs["id"], attrs["ref"], currentSegment)
				if hasAnyAttr(attrs, "start", "duration", "offset") {
					pendingTiming = true
				}
			case "clipitem", "entry":
				currentSegment = firstNonEmpty(attrs["name"], attrs["id"], attrs["producer"], currentSegment)
				pendingTiming = true
			case "name":
				if pendingTiming {
					captureSegmentText = "name"
				}
			case "comments":
				if pendingTiming {
					captureSegmentText = "comments"
				}
			case "property":
				if strings.EqualFold(attrs["name"], "vflow:segment-id") {
					captureSegmentText = "property"
				}
			case "marker":
				segmentID := firstNonEmpty(segmentIDFromText(attrs["note"]), attrs["value"], currentSegment)
				changes = appendUniqueChange(changes, "marker_note", segmentID, "marker note changed in NLE timeline", 0.95)
			case "adjust-volume", "volume":
				changes = appendUniqueChange(changes, "audio_level", currentSegment, "audio level changed in NLE timeline", 0.90)
			case "adjust-transform", "adjust-crop":
				changes = appendUniqueChange(changes, "crop_change", currentSegment, "framing transform changed in NLE timeline", 0.90)
			case "title", "generatoritem":
				currentSegment = firstNonEmpty(attrs["name"], attrs["id"], currentSegment)
				changes = appendUniqueChange(changes, "title_card", currentSegment, "title or card edited in NLE timeline", 0.85)
			case "filter-video", "filter", "effect":
				effectName := strings.ToLower(strings.Join([]string{attrs["name"], attrs["id"], attrs["uid"]}, " "))
				switch {
				case strings.Contains(effectName, "lumetri"), strings.Contains(effectName, "color"), strings.Contains(effectName, "grade"):
					changes = appendUniqueChange(changes, "color_grade", currentSegment, "color grade effect changed in NLE timeline", 0.95)
				case strings.Contains(effectName, "plugin"):
					changes = appendUniqueChange(changes, "plugin_effect", currentSegment, "plugin effect changed in NLE timeline", 0.85)
				default:
					changes = appendUniqueChange(changes, "complex_effect", currentSegment, "complex effect changed in NLE timeline", 0.80)
				}
			case "sync-clip", "mc-clip", "multicam", "ref-clip":
				currentSegment = firstNonEmpty(attrs["name"], attrs["id"], currentSegment)
				changes = appendUniqueChange(changes, "nested_timeline", currentSegment, "nested timeline reference changed in NLE timeline", 0.80)
			}
		case xml.CharData:
			if captureSegmentText != "" {
				text := strings.TrimSpace(string(t))
				if text != "" {
					currentSegment = firstNonEmpty(segmentIDFromText(text), text, currentSegment)
				}
			}
		case xml.EndElement:
			name := strings.ToLower(t.Name.Local)
			if name == "name" || name == "comments" || name == "property" {
				captureSegmentText = ""
			}
			if name == "asset-clip" || name == "clipitem" || name == "entry" || name == "title" || name == "generatoritem" {
				if pendingTiming && (name == "asset-clip" || name == "clipitem" || name == "entry") {
					changes = appendUniqueChange(changes, "clip_trim", currentSegment, "clip timing changed in NLE timeline", 0.75)
					pendingTiming = false
				}
				currentSegment = ""
			}
		}
	}
	return changes, nil
}

func parseOTIOChanges(raw []byte) ([]Change, error) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	changes := []Change{}
	var walk func(any, string)
	walk = func(value any, currentSegment string) {
		switch node := value.(type) {
		case map[string]any:
			schema := strings.ToLower(fmt.Sprint(node["OTIO_SCHEMA"]))
			if name := strings.TrimSpace(fmt.Sprint(node["name"])); name != "" && name != "<nil>" {
				currentSegment = name
			}
			if strings.HasPrefix(schema, "clip.") {
				changes = appendUniqueChange(changes, "clip_trim", currentSegment, "OTIO clip timing changed", 0.85)
			}
			if strings.HasPrefix(schema, "marker.") {
				changes = appendUniqueChange(changes, "marker_note", currentSegment, "OTIO marker changed", 0.85)
			}
			if strings.HasPrefix(schema, "effect.") {
				effectName := strings.ToLower(fmt.Sprint(node["name"]))
				if strings.Contains(effectName, "color") || strings.Contains(effectName, "grade") || strings.Contains(effectName, "lumetri") {
					changes = appendUniqueChange(changes, "color_grade", currentSegment, "OTIO color effect changed", 0.80)
				} else {
					changes = appendUniqueChange(changes, "complex_effect", currentSegment, "OTIO effect changed", 0.75)
				}
			}
			for _, child := range node {
				walk(child, currentSegment)
			}
		case []any:
			for _, child := range node {
				walk(child, currentSegment)
			}
		}
	}
	walk(doc, "")
	return changes, nil
}

func parseEDLChanges(raw []byte) []Change {
	lines := strings.Split(string(raw), "\n")
	changes := []Change{}
	segmentID := ""
	eventLine := regexp.MustCompile(`^\s*\d{3,}\s+`)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "* VFLOW-SEGMENT-ID:") {
			segmentID = strings.TrimSpace(trimmed[strings.Index(trimmed, ":")+1:])
			if len(changes) > 0 && changes[len(changes)-1].Type == "clip_trim" && changes[len(changes)-1].SegmentID == "" {
				changes[len(changes)-1].SegmentID = segmentID
			}
			continue
		}
		if eventLine.MatchString(trimmed) {
			changes = appendUniqueChange(changes, "clip_trim", segmentID, "EDL event timing changed", 0.70)
		}
	}
	return changes
}

func assignChangeIDs(changes []Change) []Change {
	for i := range changes {
		if changes[i].ID == "" {
			changes[i].ID = "change_" + strconv.Itoa(i+1)
		}
	}
	return changes
}

func appendUniqueChange(changes []Change, typ, segmentID, description string, confidence float64) []Change {
	for _, change := range changes {
		if change.Type == typ && change.SegmentID == segmentID && change.Description == description {
			return changes
		}
	}
	return append(changes, Change{
		Type:        typ,
		SegmentID:   segmentID,
		Description: description,
		Confidence:  confidence,
	})
}

func xmlAttrs(start xml.StartElement) map[string]string {
	attrs := map[string]string{}
	for _, attr := range start.Attr {
		attrs[strings.ToLower(attr.Name.Local)] = attr.Value
	}
	return attrs
}

func hasAnyAttr(attrs map[string]string, keys ...string) bool {
	for _, key := range keys {
		if attrs[key] != "" {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func segmentIDFromText(value string) string {
	const marker = "vflow:segment-id="
	idx := strings.Index(value, marker)
	if idx < 0 {
		return ""
	}
	rest := value[idx+len(marker):]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return strings.TrimSpace(rest)
	}
	return strings.Trim(fields[0], `"',;`)
}
