package nle

import "testing"

func TestClassifyRoutesKnownNLEChangesWithoutUnclassified(t *testing.T) {
	parsed, err := ParseImport("roundtrip.fcpxml", []byte(roundtripFCPXML))
	if err != nil {
		t.Fatalf("ParseImport returned error: %v", err)
	}

	diff := Classify(parsed)
	if diff.Version != "vflow-nle-diff/v1" || diff.Status != "classified" {
		t.Fatalf("unexpected diff header: %+v", diff)
	}
	if len(diff.Unclassified) != 0 {
		t.Fatalf("expected no unclassified changes, got %+v", diff.Unclassified)
	}
	for _, want := range []string{"clip_trim", "marker_note", "audio_level"} {
		if !hasChangeType(diff.SafeMerge, want) {
			t.Fatalf("safe_merge missing %q in %+v", want, diff.SafeMerge)
		}
	}
	for _, want := range []string{"title_card", "crop_change"} {
		if !hasChangeType(diff.NeedsReview, want) {
			t.Fatalf("needs_review missing %q in %+v", want, diff.NeedsReview)
		}
	}
	if !hasChangeType(diff.Blocked, "color_grade") {
		t.Fatalf("blocked missing color_grade in %+v", diff.Blocked)
	}
}

func TestApplyPlanRefusesBlockedChanges(t *testing.T) {
	diff := Classify(ImportResult{
		Version: "vflow-nle-import/v1",
		Input:   "roundtrip.fcpxml",
		Format:  "fcpxml",
		Changes: []Change{
			{ID: "safe-1", Type: "clip_trim", SegmentID: "seg_A"},
			{ID: "blocked-1", Type: "plugin_effect", SegmentID: "seg_A"},
		},
	})

	plan := PlanApply(diff, true)
	if plan.Status != "blocked" {
		t.Fatalf("expected blocked apply plan, got %+v", plan)
	}
	if len(plan.Applied) != 0 {
		t.Fatalf("blocked plan must not apply changes: %+v", plan)
	}
	if len(plan.Blocked) != 1 || plan.Blocked[0].Type != "plugin_effect" {
		t.Fatalf("expected plugin effect to remain blocked: %+v", plan)
	}
}
