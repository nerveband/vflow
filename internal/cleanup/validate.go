package cleanup

import (
	"fmt"
	"sort"
	"strings"
)

func ValidateContentEDL(edl ContentEDL) error {
	if edl.Version != "vflow-content-edl/v1" {
		return fmt.Errorf("content EDL version must be vflow-content-edl/v1")
	}
	if strings.TrimSpace(edl.Rate) == "" {
		return fmt.Errorf("content EDL rate is required")
	}
	segments := append([]DeleteSegment(nil), edl.DeleteSegments...)
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartFrame < segments[j].StartFrame
	})
	lastEnd := -1
	for _, segment := range segments {
		if strings.TrimSpace(segment.ID) == "" {
			return fmt.Errorf("delete segment id is required")
		}
		if segment.StartFrame < 0 {
			return fmt.Errorf("delete segment %s start_frame must be >= 0", segment.ID)
		}
		if segment.EndFrame <= segment.StartFrame {
			return fmt.Errorf("delete segment %s end_frame must be greater than start_frame", segment.ID)
		}
		if segment.Confidence < 0 || segment.Confidence > 1 {
			return fmt.Errorf("delete segment %s confidence must be between 0 and 1", segment.ID)
		}
		if lastEnd > segment.StartFrame {
			return fmt.Errorf("delete segment %s overlaps previous delete range", segment.ID)
		}
		lastEnd = segment.EndFrame
	}
	return nil
}
