package syncmap

import (
	"errors"
	"math"
	"sort"
)

type CorrelationResult struct {
	LagSeconds float64 `json:"lag_seconds"`
	LagSamples int     `json:"lag_samples"`
	Peak       float64 `json:"peak"`
	Confidence float64 `json:"confidence"`
}

func Correlate(reference, candidate []float64, envelopeRate int, maxLagSeconds float64) (CorrelationResult, error) {
	if len(reference) == 0 || len(candidate) == 0 {
		return CorrelationResult{}, errors.New("empty envelope")
	}
	if envelopeRate <= 0 {
		envelopeRate = EnvelopeRate
	}
	maxLag := int(math.Round(maxLagSeconds * float64(envelopeRate)))
	if maxLag <= 0 {
		maxLag = min(len(reference), len(candidate)) - 1
	}
	type score struct {
		lag int
		val float64
	}
	scores := []score{}
	best := score{val: -2}
	for lag := -maxLag; lag <= maxLag; lag++ {
		val, ok := lagCorrelation(reference, candidate, lag)
		if !ok {
			continue
		}
		scores = append(scores, score{lag: lag, val: val})
		if val > best.val {
			best = score{lag: lag, val: val}
		}
	}
	if len(scores) == 0 {
		return CorrelationResult{}, errors.New("no overlapping lag windows")
	}
	guard := max(3, envelopeRate/5)
	second := -2.0
	for _, s := range scores {
		if absInt(s.lag-best.lag) <= guard {
			continue
		}
		if s.val > second {
			second = s.val
		}
	}
	if second < -1 {
		second = -1
	}
	confidence := math.Max(0, math.Min(1, (best.val-second+1)/2))
	return CorrelationResult{
		LagSeconds: float64(best.lag) / float64(envelopeRate),
		LagSamples: best.lag,
		Peak:       best.val,
		Confidence: confidence,
	}, nil
}

func lagCorrelation(reference, candidate []float64, lag int) (float64, bool) {
	refStart, candStart := 0, 0
	if lag > 0 {
		candStart = lag
	} else if lag < 0 {
		refStart = -lag
	}
	n := min(len(reference)-refStart, len(candidate)-candStart)
	if n < 3 {
		return 0, false
	}
	dot, refMag, candMag := 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		r := reference[refStart+i]
		c := candidate[candStart+i]
		dot += r * c
		refMag += r * r
		candMag += c * c
	}
	denom := math.Sqrt(refMag * candMag)
	if denom < 1e-9 {
		return 0, false
	}
	return dot / denom, true
}

type DriftAnchor struct {
	ReferenceSeconds float64
	SourceSeconds    float64
}

func DriftPPM(anchors []DriftAnchor) float64 {
	if len(anchors) < 2 {
		return 0
	}
	sort.Slice(anchors, func(i, j int) bool { return anchors[i].ReferenceSeconds < anchors[j].ReferenceSeconds })
	first := anchors[0]
	last := anchors[len(anchors)-1]
	refDelta := last.ReferenceSeconds - first.ReferenceSeconds
	if math.Abs(refDelta) < 1e-9 {
		return 0
	}
	sourceDelta := last.SourceSeconds - first.SourceSeconds
	return ((sourceDelta / refDelta) - 1) * 1_000_000
}

func DriftWarning(ppm float64, timelineSeconds float64, fps float64) string {
	if fps <= 0 || timelineSeconds <= 0 {
		return ""
	}
	frames := math.Abs(ppm) / 1_000_000 * timelineSeconds * fps
	if frames > 1 {
		return "drift exceeds one frame across sampled timeline"
	}
	return ""
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
