package syncmap

import (
	"math"
	"testing"
)

func TestCorrelateKnownOffset(t *testing.T) {
	rate := 100
	reference := make([]float64, 1000)
	for i := 100; i < 900; i += 73 {
		reference[i] = 5
		if i+1 < len(reference) {
			reference[i+1] = 2.5
		}
	}
	lag := int(1.75 * float64(rate))
	candidate := make([]float64, len(reference)+lag)
	copy(candidate[lag:], reference)
	result, err := Correlate(reference, candidate, rate, 3)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(result.LagSeconds-1.75) > 0.02 {
		t.Fatalf("lag = %.3f, want 1.75", result.LagSeconds)
	}
	if result.Confidence < 0.5 {
		t.Fatalf("confidence = %.3f, want high", result.Confidence)
	}
}

func TestDriftPPM(t *testing.T) {
	got := DriftPPM([]DriftAnchor{{ReferenceSeconds: 0, SourceSeconds: 0}, {ReferenceSeconds: 1000, SourceSeconds: 1000.1}})
	if got < 99 || got > 101 {
		t.Fatalf("drift ppm = %.3f, want about 100", got)
	}
}
