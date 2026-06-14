package audit

import "testing"

func TestRunScoresHardenedCLIAtOrAboveThreshold(t *testing.T) {
	report := Run("../..")
	if report.Threshold != 85 {
		t.Fatalf("threshold = %d, want 85", report.Threshold)
	}
	if report.Score < report.Threshold {
		t.Fatalf("score = %d threshold = %d checks = %+v", report.Score, report.Threshold, report.Checks)
	}
	if report.Status != "pass" {
		t.Fatalf("status = %q", report.Status)
	}
}

func TestRunIncludesProviderAdapterEvidence(t *testing.T) {
	report := Run("../..")
	found := false
	for _, check := range report.Checks {
		if check.ID == "live_stt_adapters" {
			found = true
			if !check.Passed {
				t.Fatalf("live adapter check did not pass: %+v", check)
			}
		}
	}
	if !found {
		t.Fatalf("missing live_stt_adapters check")
	}
}
