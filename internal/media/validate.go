package media

import (
	"fmt"
	"strings"
)

func ValidateSourceReviewArtifact(artifact SourceReviewArtifact) error {
	if artifact.Version != "vflow-source-media-review/v1" {
		return fmt.Errorf("source media review version must be vflow-source-media-review/v1")
	}
	if len(artifact.Sources) == 0 {
		return fmt.Errorf("source media review must include at least one source")
	}
	for i, source := range artifact.Sources {
		if err := ValidateSourceReview(source); err != nil {
			return fmt.Errorf("source %d invalid: %w", i, err)
		}
	}
	return nil
}

func ValidateSourceReview(review SourceReview) error {
	if review.Version != "vflow-source-media-review/v1" {
		return fmt.Errorf("source version must be vflow-source-media-review/v1")
	}
	if strings.TrimSpace(review.Source) == "" {
		return fmt.Errorf("source path is required")
	}
	if review.Width <= 0 || review.Height <= 0 {
		return fmt.Errorf("source dimensions must be positive")
	}
	if review.DurationSeconds <= 0 {
		return fmt.Errorf("source duration_seconds must be positive")
	}
	if strings.TrimSpace(review.FrameRate) == "" {
		return fmt.Errorf("source frame_rate is required")
	}
	if strings.TrimSpace(review.Timebase) == "" {
		return fmt.Errorf("source timebase is required")
	}
	if strings.TrimSpace(review.Codec) == "" {
		return fmt.Errorf("source codec is required")
	}
	switch review.VariableFrameRateStatus {
	case "unknown", "likely_cfr", "possible_vfr":
	default:
		return fmt.Errorf("source variable_frame_rate_status is invalid")
	}
	if len(review.RepresentativeFramePlan) == 0 {
		return fmt.Errorf("source representative_frame_plan is required")
	}
	for _, audio := range review.AudioStreams {
		if audio.Index < 0 {
			return fmt.Errorf("audio stream index must be >= 0")
		}
		if strings.TrimSpace(audio.Codec) == "" {
			return fmt.Errorf("audio stream codec is required")
		}
		if audio.Channels < 0 {
			return fmt.Errorf("audio stream channels must be >= 0")
		}
	}
	return nil
}
