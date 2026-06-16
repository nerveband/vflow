package cleanup

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type inputDelete struct {
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

func ImportDeleteSegments(raw []byte, rate string) (ContentEDL, error) {
	var input []inputDelete
	if err := json.Unmarshal(raw, &input); err != nil {
		return ContentEDL{}, err
	}
	fps := frameRate(rate)
	edl := ContentEDL{Version: "vflow-content-edl/v1", Rate: rate}
	for i, seg := range input {
		edl.DeleteSegments = append(edl.DeleteSegments, DeleteSegment{
			ID:          fmt.Sprintf("del_%06d", i+1),
			StartFrame:  int(math.Round(seg.Start * fps)),
			EndFrame:    int(math.Round(seg.End * fps)),
			Reason:      seg.Reason,
			Confidence:  seg.Confidence,
			SourceStart: seg.Start,
			SourceEnd:   seg.End,
		})
	}
	if err := ValidateContentEDL(edl); err != nil {
		return ContentEDL{}, err
	}
	return edl, nil
}

func WriteContentEDL(projectPath string, edl ContentEDL) error {
	if err := ValidateContentEDL(edl); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(edl, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(projectPath, "decisions", "content-edl.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, append(raw, '\n'), 0o644)
}

func ReadContentEDL(projectPath string) (ContentEDL, error) {
	raw, err := os.ReadFile(filepath.Join(projectPath, "decisions", "content-edl.json"))
	if err != nil {
		return ContentEDL{}, err
	}
	var edl ContentEDL
	if err := json.Unmarshal(raw, &edl); err != nil {
		return edl, err
	}
	return edl, ValidateContentEDL(edl)
}

func frameRate(rate string) float64 {
	if rate == "" {
		return 30
	}
	if left, right, ok := strings.Cut(rate, "/"); ok {
		num, _ := strconv.ParseFloat(left, 64)
		den, _ := strconv.ParseFloat(right, 64)
		if den != 0 {
			return num / den
		}
	}
	value, err := strconv.ParseFloat(rate, 64)
	if err == nil && value > 0 {
		return value
	}
	return 30
}
