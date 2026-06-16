package nle

func Classify(result ImportResult) DiffResult {
	diff := DiffResult{
		Version:      "vflow-nle-diff/v1",
		Status:       "classified",
		Import:       result.Input,
		Format:       result.Format,
		SafeMerge:    []Change{},
		NeedsReview:  []Change{},
		Blocked:      []Change{},
		Unclassified: []Change{},
	}
	for _, change := range result.Changes {
		if requiresSegmentIdentity(change.Type) && change.SegmentID == "" {
			diff.Blocked = append(diff.Blocked, Change{
				ID:          firstNonEmpty(change.ID, "missing_sidecar"),
				Type:        "missing_sidecar",
				Description: "NLE change is missing vflow sidecar segment identity",
				Confidence:  change.Confidence,
			})
			continue
		}
		switch classifyChange(change.Type) {
		case "safe_merge":
			diff.SafeMerge = append(diff.SafeMerge, change)
		case "needs_review":
			diff.NeedsReview = append(diff.NeedsReview, change)
		case "blocked":
			diff.Blocked = append(diff.Blocked, change)
		default:
			diff.Unclassified = append(diff.Unclassified, change)
		}
	}
	return diff
}

func requiresSegmentIdentity(changeType string) bool {
	switch changeType {
	case "clip_trim", "deleted_source_segment", "added_source_segment", "marker_note", "audio_level", "dialogue_shift", "speed_change", "media_replace", "crop_change", "title_card", "transform", "color_grade", "complex_effect", "nested_timeline", "plugin_effect", "keyframed_transform":
		return true
	default:
		return false
	}
}

func classifyChange(changeType string) string {
	switch changeType {
	case "clip_trim", "deleted_source_segment", "added_source_segment", "marker_note", "caption_text", "broll_timing", "audio_level":
		return "safe_merge"
	case "dialogue_shift", "speed_change", "media_replace", "crop_change", "title_card", "transform":
		return "needs_review"
	case "color_grade", "complex_effect", "nested_timeline", "plugin_effect", "keyframed_transform", "missing_sidecar":
		return "blocked"
	default:
		return "unclassified"
	}
}
