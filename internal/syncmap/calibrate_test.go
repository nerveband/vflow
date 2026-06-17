package syncmap

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCalibratePlansMultipleVotingWindows(t *testing.T) {
	dir := t.TempDir()
	report, err := Calibrate(context.Background(), CalibrationOptions{
		ReferenceSourceID: "a",
		Sources: []SourceInput{
			{ID: "a", Path: "a.mp4", WindowStart: 10, WindowDuration: 30},
			{ID: "b", Path: "b.mp4", WindowStart: 10, WindowDuration: 30},
		},
		OutputPath: filepath.Join(dir, "media-sync-map.json"),
		ProofDir:   filepath.Join(dir, "proof"),
		Windows:    3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.ExtractPlans) != 6 {
		t.Fatalf("extract plans = %d, want 6", len(report.ExtractPlans))
	}
	if report.ExtractPlans[0].Start != 10 || report.ExtractPlans[1].Start != 70 || report.ExtractPlans[2].Start != 130 {
		t.Fatalf("unexpected window starts: %.1f %.1f %.1f", report.ExtractPlans[0].Start, report.ExtractPlans[1].Start, report.ExtractPlans[2].Start)
	}
	if filepath.Base(report.ExtractPlans[0].Output) != "a-w01.s16le" || filepath.Base(report.WaveformPlans[5].Output) != "b-w03.waveform.png" {
		t.Fatalf("unexpected proof paths: %#v %#v", report.ExtractPlans[0], report.WaveformPlans[5])
	}
	if report.SyncMap.Sources[1].Warnings == nil {
		t.Fatalf("expected non-reference source to disclose voting mode")
	}
}

func TestVoteCorrelationsUsesMedianLagAndAverageConfidence(t *testing.T) {
	got := voteCorrelations([]CorrelationResult{
		{LagSeconds: 1.0, Confidence: 0.90},
		{LagSeconds: 9.0, Confidence: 0.20},
		{LagSeconds: 1.1, Confidence: 0.80},
	})
	if got.LagSeconds < 1.09 || got.LagSeconds > 1.11 {
		t.Fatalf("lag = %.3f, want median 1.1", got.LagSeconds)
	}
	if got.Confidence < 0.63 || got.Confidence > 0.64 {
		t.Fatalf("confidence = %.3f, want average confidence", got.Confidence)
	}
}
