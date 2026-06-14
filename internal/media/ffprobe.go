package media

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type AudioStream struct {
	Index      int    `json:"index"`
	Codec      string `json:"codec"`
	SampleRate string `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
}

type SourceReview struct {
	Version                 string        `json:"version"`
	Source                  string        `json:"source"`
	Width                   int           `json:"width"`
	Height                  int           `json:"height"`
	DurationSeconds         float64       `json:"duration_seconds"`
	FrameRate               string        `json:"frame_rate"`
	Timebase                string        `json:"timebase"`
	Codec                   string        `json:"codec"`
	AudioStreams            []AudioStream `json:"audio_streams"`
	VariableFrameRateStatus string        `json:"variable_frame_rate_status"`
	Warnings                []string      `json:"warnings,omitempty"`
	RepresentativeFramePlan []string      `json:"representative_frame_plan"`
	ProbeCommand            []string      `json:"probe_command,omitempty"`
}

type ffprobeOutput struct {
	Streams []struct {
		Index        int    `json:"index"`
		CodecName    string `json:"codec_name"`
		CodecType    string `json:"codec_type"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		RFrameRate   string `json:"r_frame_rate"`
		AvgFrameRate string `json:"avg_frame_rate"`
		TimeBase     string `json:"time_base"`
		Duration     string `json:"duration"`
		SampleRate   string `json:"sample_rate"`
		Channels     int    `json:"channels"`
	} `json:"streams"`
	Format struct {
		Filename string `json:"filename"`
		Duration string `json:"duration"`
		Size     string `json:"size"`
	} `json:"format"`
}

func ParseFFProbe(raw []byte, source string) (SourceReview, error) {
	var parsed ffprobeOutput
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return SourceReview{}, err
	}
	review := SourceReview{
		Version:                 "vflow-source-media-review/v1",
		Source:                  filepath.ToSlash(source),
		VariableFrameRateStatus: "unknown",
		RepresentativeFramePlan: []string{"first_frame", "midpoint_frame", "last_frame"},
	}
	for _, stream := range parsed.Streams {
		switch stream.CodecType {
		case "video":
			if review.Width == 0 {
				review.Width = stream.Width
				review.Height = stream.Height
				review.Codec = stream.CodecName
				review.FrameRate = firstNonEmpty(stream.AvgFrameRate, stream.RFrameRate)
				review.Timebase = stream.TimeBase
				review.DurationSeconds, _ = strconv.ParseFloat(firstNonEmpty(stream.Duration, parsed.Format.Duration), 64)
				if stream.AvgFrameRate != "" && stream.RFrameRate != "" && stream.AvgFrameRate != stream.RFrameRate {
					review.VariableFrameRateStatus = "possible_vfr"
					review.Warnings = append(review.Warnings, "average frame rate differs from nominal frame rate")
				} else {
					review.VariableFrameRateStatus = "likely_cfr"
				}
			}
		case "audio":
			review.AudioStreams = append(review.AudioStreams, AudioStream{
				Index:      stream.Index,
				Codec:      stream.CodecName,
				SampleRate: stream.SampleRate,
				Channels:   stream.Channels,
			})
		}
	}
	if review.DurationSeconds == 0 {
		review.DurationSeconds, _ = strconv.ParseFloat(parsed.Format.Duration, 64)
	}
	return review, nil
}

func ProbeFile(ffprobePath, source string) (SourceReview, error) {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	args := []string{"-v", "error", "-show_format", "-show_streams", "-output_format", "json", source}
	raw, err := exec.Command(ffprobePath, args...).Output()
	if err != nil {
		return SourceReview{}, err
	}
	review, err := ParseFFProbe(raw, source)
	if err != nil {
		return SourceReview{}, err
	}
	review.ProbeCommand = append([]string{ffprobePath}, args...)
	return review, nil
}

func WriteReview(projectPath string, review SourceReview) error {
	raw, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectPath, "source-media-review.json"), append(raw, '\n'), 0o644)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
