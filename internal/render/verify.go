package render

import (
	"encoding/json"
	"math"
	"os/exec"
	"strconv"
)

type Verification struct {
	Status          string   `json:"status"`
	Render          string   `json:"render"`
	Width           int      `json:"width,omitempty"`
	Height          int      `json:"height,omitempty"`
	DurationSeconds float64  `json:"duration_seconds,omitempty"`
	VideoCodec      string   `json:"video_codec,omitempty"`
	AudioStreams    int      `json:"audio_streams"`
	FrameCount      int      `json:"frame_count,omitempty"`
	Issues          []string `json:"issues,omitempty"`
}

type VerifyResult = Verification

type VerifyOptions struct {
	Render                  string
	ExpectedWidth           int
	ExpectedHeight          int
	ExpectedDurationSeconds float64
	DurationTolerance       float64
}

type probeOutput struct {
	Streams []struct {
		CodecName string `json:"codec_name"`
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Duration  string `json:"duration"`
		NBFrames  string `json:"nb_frames"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func VerifyProbe(raw []byte, opts VerifyOptions) (Verification, error) {
	var parsed probeOutput
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Verification{}, err
	}
	if opts.DurationTolerance <= 0 {
		opts.DurationTolerance = 0.25
	}
	out := Verification{Status: "valid", Render: opts.Render}
	for _, stream := range parsed.Streams {
		switch stream.CodecType {
		case "video":
			if out.Width == 0 {
				out.Width = stream.Width
				out.Height = stream.Height
				out.VideoCodec = stream.CodecName
				out.DurationSeconds, _ = strconv.ParseFloat(firstNonEmpty(stream.Duration, parsed.Format.Duration), 64)
				out.FrameCount, _ = strconv.Atoi(stream.NBFrames)
			}
		case "audio":
			out.AudioStreams++
		}
	}
	if out.DurationSeconds == 0 {
		out.DurationSeconds, _ = strconv.ParseFloat(parsed.Format.Duration, 64)
	}
	if opts.ExpectedWidth > 0 && opts.ExpectedHeight > 0 && (out.Width != opts.ExpectedWidth || out.Height != opts.ExpectedHeight) {
		out.Issues = append(out.Issues, "resolution_mismatch")
	}
	if opts.ExpectedDurationSeconds > 0 && math.Abs(out.DurationSeconds-opts.ExpectedDurationSeconds) > opts.DurationTolerance {
		out.Issues = append(out.Issues, "duration_mismatch")
	}
	if out.AudioStreams == 0 {
		out.Issues = append(out.Issues, "missing_audio")
	}
	if len(out.Issues) > 0 {
		out.Status = "invalid"
	}
	return out, nil
}

func VerifyFile(ffprobePath string, opts VerifyOptions) (Verification, error) {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	raw, err := exec.Command(ffprobePath, "-v", "error", "-show_format", "-show_streams", "-output_format", "json", opts.Render).Output()
	if err != nil {
		return Verification{}, err
	}
	return VerifyProbe(raw, opts)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
