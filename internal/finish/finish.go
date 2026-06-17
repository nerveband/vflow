package finish

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	vframing "github.com/nerveband/vflow/internal/framing"
	vtranscript "github.com/nerveband/vflow/internal/transcript"
)

type Adapter struct {
	ID                string   `json:"id"`
	Task              string   `json:"task"`
	Name              string   `json:"name"`
	SupportedTasks    []string `json:"supported_tasks"`
	RequiredTools     []string `json:"required_tools"`
	ContractSchemaIDs []string `json:"contract_schema_ids"`
	Invocation        string   `json:"invocation_template"`
	Verification      bool     `json:"verification"`
	Available         bool     `json:"available"`
	MissingTools      []string `json:"missing_tools,omitempty"`
}

type Suggestion struct {
	SchemaVersion        string    `json:"schema_version"`
	Task                 string    `json:"task"`
	ContractSchema       string    `json:"contract_schema"`
	ContractExample      any       `json:"contract_example"`
	Recommendation       Adapter   `json:"recommendation"`
	Invocation           string    `json:"invocation"`
	Alternatives         []Adapter `json:"alternatives"`
	DetectedCapabilities []Adapter `json:"detected_capabilities"`
	MissingToolHints     []string  `json:"missing_tool_hints"`
	Safety               []string  `json:"safety"`
}

type VerifyResult struct {
	SchemaVersion   string                `json:"schema_version"`
	Task            string                `json:"task"`
	Status          string                `json:"status"`
	Checks          []Check               `json:"checks"`
	ReviewItems     []vframing.ReviewItem `json:"review_items"`
	ReviewQueuePath string                `json:"review_queue_path,omitempty"`
}

type Check struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

type Brand struct {
	Version         string         `json:"version"`
	CaptionStyles   map[string]any `json:"caption_styles"`
	LayoutIDs       []string       `json:"layout_ids"`
	LoudnessTargets map[string]any `json:"loudness_targets"`
	SafeMargins     map[string]any `json:"safe_margins"`
}

type speakerPerson struct {
	DisplayName string
	Title       string
}

func DetectAdapters() []Adapter {
	adapters := []Adapter{
		{ID: "ffmpeg-subtitles", Task: "captions", Name: "ffmpeg subtitles sidecar/burn-in", SupportedTasks: []string{"captions"}, RequiredTools: []string{"ffmpeg"}, ContractSchemaIDs: []string{"caption-cues.schema.json", "brand.schema.json"}, Invocation: "ffmpeg -i input.mp4 -i captions.srt -c:v copy -c:a copy output.mp4", Verification: true},
		{ID: "nle-caption-sidecar", Task: "captions", Name: "NLE caption sidecar adapter", SupportedTasks: []string{"captions"}, RequiredTools: []string{}, ContractSchemaIDs: []string{"caption-cues.schema.json", "brand.schema.json"}, Invocation: "adapter writes caption sidecar from vflow-caption-cues/v1", Verification: true},
		{ID: "ffmpeg-sidechain-loudnorm", Task: "audio", Name: "ffmpeg sidechain duck + loudnorm", SupportedTasks: []string{"audio"}, RequiredTools: []string{"ffmpeg", "ffprobe"}, ContractSchemaIDs: []string{"audio-intent.schema.json", "brand.schema.json"}, Invocation: "ffmpeg -i speech.wav -i bed.wav -filter_complex sidechaincompress,loudnorm output.wav", Verification: true},
		{ID: "compositor-adapter", Task: "supers", Name: "external compositor adapter", SupportedTasks: []string{"supers"}, RequiredTools: []string{}, ContractSchemaIDs: []string{"super-cards.schema.json", "brand.schema.json", "speaker-map.schema.json"}, Invocation: "compositor-adapter --spec decisions/super-cards.json --brand brand.json", Verification: true},
		{ID: "motion-expression-adapter", Task: "motion", Name: "renderer expression adapter", SupportedTasks: []string{"motion"}, RequiredTools: []string{}, ContractSchemaIDs: []string{"motion-ramp.schema.json", "framing-presets.schema.json"}, Invocation: "renderer-adapter --motion decisions/motion-ramp.json --presets calibration/framing-presets.json", Verification: true},
		{ID: "mixer-sfx-adapter", Task: "sfx", Name: "external mixer SFX adapter", SupportedTasks: []string{"sfx"}, RequiredTools: []string{}, ContractSchemaIDs: []string{"sfx-cues.schema.json"}, Invocation: "mixer-adapter --spec decisions/sfx-cues.json", Verification: true},
		{ID: "broll-nle-adapter", Task: "broll", Name: "NLE b-roll placeholder adapter", SupportedTasks: []string{"broll"}, RequiredTools: []string{}, ContractSchemaIDs: []string{"broll-plan.schema.json"}, Invocation: "nle-adapter --spec decisions/broll-plan.json", Verification: true},
	}
	for i := range adapters {
		for _, tool := range adapters[i].RequiredTools {
			if _, err := exec.LookPath(tool); err != nil {
				adapters[i].MissingTools = append(adapters[i].MissingTools, tool)
			}
		}
		adapters[i].Available = len(adapters[i].MissingTools) == 0
	}
	return adapters
}

func Suggest(task, project string) (Suggestion, error) {
	adapters := adaptersForTask(task)
	if len(adapters) == 0 {
		return Suggestion{}, fmt.Errorf("unsupported finishing task %q", task)
	}
	sort.SliceStable(adapters, func(i, j int) bool {
		if adapters[i].Available == adapters[j].Available {
			return adapters[i].ID < adapters[j].ID
		}
		return adapters[i].Available
	})
	recommendation := adapters[0]
	missing := []string{}
	for _, adapter := range adapters {
		for _, tool := range adapter.MissingTools {
			missing = append(missing, fmt.Sprintf("install %s for %s", tool, adapter.ID))
		}
	}
	return Suggestion{
		SchemaVersion:        "vflow-suggestion/v1",
		Task:                 task,
		ContractSchema:       contractSchema(task),
		ContractExample:      contractExample(task),
		Recommendation:       recommendation,
		Invocation:           renderInvocation(task, project, recommendation),
		Alternatives:         adapters[1:],
		DetectedCapabilities: adapters,
		MissingToolHints:     missing,
		Safety: []string{
			"vflow does not render, mix, composite, or burn captions for finishing tasks",
			"agents run external tools and return artifacts for vflow verify",
			"use approved preset IDs and brand tokens instead of invented visual values",
		},
	}, nil
}

func Verify(task, project, specPath, outputPath string) (VerifyResult, error) {
	switch task {
	case "captions":
		return verifyCaptions(project, specPath, outputPath)
	case "audio":
		return verifyAudio(project, specPath, outputPath)
	case "supers":
		return verifySupers(project, specPath)
	case "motion":
		return verifyMotion(project, specPath, outputPath)
	case "sfx", "broll":
		return verifySpecOnly(task, specPath)
	default:
		return VerifyResult{}, fmt.Errorf("unsupported finishing task %q", task)
	}
}

func adaptersForTask(task string) []Adapter {
	var out []Adapter
	for _, adapter := range DetectAdapters() {
		if adapter.Task == task {
			out = append(out, adapter)
		}
	}
	return out
}

func verifyCaptions(project, specPath, outputPath string) (VerifyResult, error) {
	var spec struct {
		Version  string `json:"version"`
		WordsRef string `json:"words_ref"`
		StyleID  string `json:"style_id"`
		MaxDrift int64  `json:"max_drift_frames"`
		Cues     []struct {
			ID         string   `json:"id"`
			Text       string   `json:"text"`
			WordIDs    []string `json:"word_ids"`
			StartFrame int64    `json:"start_frame"`
			EndFrame   int64    `json:"end_frame"`
		} `json:"cues"`
	}
	if err := readJSON(specPath, &spec); err != nil {
		return VerifyResult{}, err
	}
	brand, _ := ReadBrand(project)
	wordsPath := filepath.Join(project, spec.WordsRef)
	if spec.WordsRef == "" {
		wordsPath = filepath.Join(project, "transcript", "words.json")
	}
	words, err := vtranscript.ReadWords(filepath.Dir(filepath.Dir(wordsPath)))
	if err != nil {
		return VerifyResult{}, err
	}
	byID := map[string]vtranscript.Word{}
	for _, word := range words.Words {
		byID[word.ID] = word
	}
	maxDrift := spec.MaxDrift
	if maxDrift == 0 {
		maxDrift = 2
	}
	result := baseResult("captions")
	if spec.Version != "vflow-caption-cues/v1" {
		addFailure(&result, "caption_contract", "caption spec version must be vflow-caption-cues/v1", 0, 1)
	}
	if spec.StyleID == "" || brand.Version != "" && brand.CaptionStyles != nil && brand.CaptionStyles[spec.StyleID] == nil {
		addFailure(&result, "caption_style_token", "caption style_id is not defined in brand.json", 0, 1)
	}
	for _, cue := range spec.Cues {
		if len(cue.WordIDs) == 0 {
			addFailure(&result, "caption_word_anchor", "caption cue has no word_ids", cue.StartFrame, cue.EndFrame)
			continue
		}
		start, end := int64(math.MaxInt64), int64(0)
		for _, id := range cue.WordIDs {
			word, ok := byID[id]
			if !ok {
				addFailure(&result, "caption_word_anchor", "caption cue references missing word_id "+id, cue.StartFrame, cue.EndFrame)
				continue
			}
			if word.StartFrame < start {
				start = word.StartFrame
			}
			if word.EndFrame > end {
				end = word.EndFrame
			}
		}
		if start != math.MaxInt64 && (abs64(cue.StartFrame-start) > maxDrift || abs64(cue.EndFrame-end) > maxDrift) {
			addFailure(&result, "caption_timing_drift", fmt.Sprintf("caption cue %s drifts beyond %d frames", cue.ID, maxDrift), cue.StartFrame, cue.EndFrame)
		}
	}
	if outputPath != "" {
		verifyCaptionOutput(project, outputPath, spec.Cues, maxDrift, words.Rate, &result)
	}
	return finishResult(result), nil
}

func verifyCaptionOutput(project, outputPath string, cues []struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	WordIDs    []string `json:"word_ids"`
	StartFrame int64    `json:"start_frame"`
	EndFrame   int64    `json:"end_frame"`
}, maxDrift int64, rate string, result *VerifyResult) {
	path := resolveProjectPath(project, outputPath)
	info, err := os.Stat(path)
	if err != nil {
		addFailure(result, "caption_output_missing", "caption output file is missing", 0, 1)
		return
	}
	if info.Size() == 0 {
		addFailure(result, "caption_output_empty", "caption output file is empty", 0, 1)
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		addFailure(result, "caption_output_read_failed", err.Error(), 0, 1)
		return
	}
	outputCues, err := parseSRTCues(string(raw), rate)
	if err != nil {
		addFailure(result, "caption_output_invalid", err.Error(), 0, 1)
		return
	}
	if len(outputCues) != len(cues) {
		addFailure(result, "caption_output_cue_count", fmt.Sprintf("caption output has %d cues; spec has %d", len(outputCues), len(cues)), 0, 1)
		return
	}
	for i, cue := range cues {
		outputCue := outputCues[i]
		if abs64(outputCue.StartFrame-cue.StartFrame) > maxDrift || abs64(outputCue.EndFrame-cue.EndFrame) > maxDrift {
			addFailure(result, "caption_output_timing_drift", fmt.Sprintf("caption output cue %s drifts beyond %d frames", cue.ID, maxDrift), cue.StartFrame, cue.EndFrame)
		}
		if normalizedCaptionText(outputCue.Text) != "" && normalizedCaptionText(cue.Text) != "" && normalizedCaptionText(outputCue.Text) != normalizedCaptionText(cue.Text) {
			addFailure(result, "caption_output_text_mismatch", fmt.Sprintf("caption output cue %s text does not match spec", cue.ID), cue.StartFrame, cue.EndFrame)
		}
	}
}

func verifyAudio(project, specPath, outputPath string) (VerifyResult, error) {
	var spec struct {
		Version            string  `json:"version"`
		LoudnessTargetLUFS float64 `json:"loudness_target_lufs"`
		SpeechSegments     []any   `json:"speech_segments"`
	}
	if err := readJSON(specPath, &spec); err != nil {
		return VerifyResult{}, err
	}
	var report struct {
		IntegratedLUFS  float64 `json:"integrated_lufs"`
		TruePeakDB      float64 `json:"true_peak_db"`
		Clipping        bool    `json:"clipping"`
		TimingPreserved bool    `json:"timing_preserved"`
	}
	if outputPath != "" {
		if err := readJSON(outputPath, &report); err != nil {
			return VerifyResult{}, err
		}
	}
	result := baseResult("audio")
	if spec.Version != "vflow-audio-intent/v1" {
		addFailure(&result, "audio_contract", "audio spec version must be vflow-audio-intent/v1", 0, 1)
	}
	if len(spec.SpeechSegments) == 0 {
		addFailure(&result, "audio_speech_anchors", "audio intent must include speech segment anchors", 0, 1)
	}
	target := spec.LoudnessTargetLUFS
	if target == 0 {
		if brand, err := ReadBrand(project); err == nil {
			if raw, ok := brand.LoudnessTargets["integrated_lufs"].(float64); ok {
				target = raw
			}
		}
	}
	if outputPath != "" {
		if math.Abs(report.IntegratedLUFS-target) > 2 {
			addFailure(&result, "audio_loudness_range", "integrated loudness is outside target range", 0, 1)
		}
		if report.Clipping || report.TruePeakDB > 0 {
			addFailure(&result, "audio_clipping", "audio report indicates clipping or positive true peak", 0, 1)
		}
		if !report.TimingPreserved {
			addFailure(&result, "audio_timing_preservation", "audio processing did not preserve timing", 0, 1)
		}
	}
	return finishResult(result), nil
}

func verifySupers(project, specPath string) (VerifyResult, error) {
	var spec struct {
		Version       string `json:"version"`
		BrandRef      string `json:"brand_ref"`
		SpeakerMapRef string `json:"speaker_map_ref"`
		Items         []struct {
			ID              string `json:"id"`
			LayoutID        string `json:"layout_id"`
			Text            string `json:"text"`
			SpeakerLabel    string `json:"speaker_label"`
			SafeMarginToken string `json:"safe_margin_token"`
			StartFrame      int64  `json:"start_frame"`
			EndFrame        int64  `json:"end_frame"`
		} `json:"items"`
	}
	if err := readJSON(specPath, &spec); err != nil {
		return VerifyResult{}, err
	}
	brand, _ := ReadBrand(project)
	speakers := readSpeakerPeople(project, spec.SpeakerMapRef)
	result := baseResult("supers")
	if spec.Version != "vflow-super-cards/v1" {
		addFailure(&result, "super-card_contract", "super/card spec version must be vflow-super-cards/v1", 0, 1)
	}
	if brand.Version == "" {
		addFailure(&result, "super-card_brand", "brand.json is required for supers verification", 0, 1)
	}
	for _, item := range spec.Items {
		if item.LayoutID == "" || !contains(brand.LayoutIDs, item.LayoutID) {
			addFailure(&result, "super-card_layout_token", "layout_id is not defined in brand.json", item.StartFrame, item.EndFrame)
		}
		if item.SafeMarginToken != "" && brand.SafeMargins[item.SafeMarginToken] == nil {
			addFailure(&result, "super-card_safe_margin", "safe_margin_token is not defined in brand.json", item.StartFrame, item.EndFrame)
		}
		if item.SpeakerLabel != "" {
			person, ok := speakers[item.SpeakerLabel]
			if !ok {
				addFailure(&result, "super-card_speaker", "speaker_label is not defined in speaker map", item.StartFrame, item.EndFrame)
				continue
			}
			if person.DisplayName != "" && !containsText(item.Text, person.DisplayName) {
				addFailure(&result, "super-card_spelling", "super/card text does not match speaker map display name", item.StartFrame, item.EndFrame)
			}
		}
	}
	return finishResult(result), nil
}

func verifyMotion(project, specPath, outputPath string) (VerifyResult, error) {
	var spec struct {
		Version     string `json:"version"`
		FromPreset  string `json:"from_preset_id"`
		ToPreset    string `json:"to_preset_id"`
		StartFrame  int64  `json:"start_frame"`
		EndFrame    int64  `json:"end_frame"`
		FrameDiffOK bool   `json:"frame_diff_confirmed"`
	}
	if err := readJSON(specPath, &spec); err != nil {
		return VerifyResult{}, err
	}
	var report struct {
		FrameDiffOK bool `json:"frame_diff_confirmed"`
	}
	if outputPath != "" {
		if err := readJSON(outputPath, &report); err != nil {
			return VerifyResult{}, err
		}
		spec.FrameDiffOK = report.FrameDiffOK
	}
	presetIDs := readPresetIDs(project)
	result := baseResult("motion")
	if spec.Version != "vflow-motion-ramp/v1" {
		addFailure(&result, "motion_contract", "motion spec version must be vflow-motion-ramp/v1", spec.StartFrame, spec.EndFrame)
	}
	if spec.FromPreset == "" || spec.ToPreset == "" || !presetIDs[spec.FromPreset] || !presetIDs[spec.ToPreset] {
		addFailure(&result, "motion_preset_tokens", "motion ramp must reference approved framing preset IDs", spec.StartFrame, spec.EndFrame)
	}
	if outputPath != "" && !spec.FrameDiffOK {
		addFailure(&result, "motion_frozen_frame", "frame-diff did not confirm real movement", spec.StartFrame, spec.EndFrame)
	}
	return finishResult(result), nil
}

func verifySpecOnly(task, specPath string) (VerifyResult, error) {
	var spec map[string]any
	if err := readJSON(specPath, &spec); err != nil {
		return VerifyResult{}, err
	}
	result := baseResult(task)
	if spec["version"] == "" {
		addFailure(&result, task+"_contract", "spec must include a version", 0, 1)
	}
	return finishResult(result), nil
}

func ReadBrand(project string) (Brand, error) {
	var brand Brand
	err := readJSON(filepath.Join(project, "brand.json"), &brand)
	return brand, err
}

func baseResult(task string) VerifyResult {
	return VerifyResult{SchemaVersion: "vflow-verification/v1", Task: task, Status: "passed", Checks: []Check{}, ReviewItems: []vframing.ReviewItem{}}
}

func finishResult(result VerifyResult) VerifyResult {
	if len(result.ReviewItems) > 0 {
		result.Status = "failed"
	} else {
		result.Checks = append(result.Checks, Check{ID: result.Task + "_conformance", Status: "passed", Message: "spec and output conform to vflow control-plane checks"})
	}
	return result
}

func addFailure(result *VerifyResult, code, message string, startFrame, endFrame int64) {
	result.Checks = append(result.Checks, Check{ID: code, Status: "failed", Message: message, Severity: "review"})
	result.ReviewItems = append(result.ReviewItems, vframing.ReviewItem{Code: code, Severity: "review", Message: message, EventID: result.Task, StartFrame: startFrame, EndFrame: endFrame, PresetID: "n/a"})
}

func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

type srtCue struct {
	StartFrame int64
	EndFrame   int64
	Text       string
}

func parseSRTCues(raw, rate string) ([]srtCue, error) {
	blocks := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n\n")
	cues := []srtCue{}
	timeLine := regexp.MustCompile(`^\s*(\d{2}:\d{2}:\d{2}[,.]\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}[,.]\d{3})`)
	for _, block := range blocks {
		lines := nonEmptyLines(block)
		if len(lines) == 0 {
			continue
		}
		timeIndex := 0
		if len(lines) > 1 && !strings.Contains(lines[0], "-->") {
			timeIndex = 1
		}
		if timeIndex >= len(lines) {
			continue
		}
		matches := timeLine.FindStringSubmatch(lines[timeIndex])
		if matches == nil {
			return nil, fmt.Errorf("invalid SRT cue timing line %q", lines[timeIndex])
		}
		start, err := srtTimestampToFrames(matches[1], rate)
		if err != nil {
			return nil, err
		}
		end, err := srtTimestampToFrames(matches[2], rate)
		if err != nil {
			return nil, err
		}
		text := ""
		if timeIndex+1 < len(lines) {
			text = strings.Join(lines[timeIndex+1:], " ")
		}
		cues = append(cues, srtCue{StartFrame: start, EndFrame: end, Text: text})
	}
	if len(cues) == 0 {
		return nil, fmt.Errorf("caption output contains no SRT cues")
	}
	return cues, nil
}

func nonEmptyLines(block string) []string {
	out := []string{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func srtTimestampToFrames(value, rate string) (int64, error) {
	parts := strings.Split(strings.ReplaceAll(value, ",", "."), ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid SRT timestamp %q", value)
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid SRT timestamp %q", value)
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid SRT timestamp %q", value)
	}
	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid SRT timestamp %q", value)
	}
	totalSeconds := float64(hours*3600+minutes*60) + seconds
	return int64(math.Round(totalSeconds * frameRate(rate))), nil
}

func frameRate(rate string) float64 {
	if rate == "" {
		return 30
	}
	if strings.Contains(rate, "/") {
		parts := strings.SplitN(rate, "/", 2)
		numerator, numeratorErr := strconv.ParseFloat(parts[0], 64)
		denominator, denominatorErr := strconv.ParseFloat(parts[1], 64)
		if numeratorErr == nil && denominatorErr == nil && denominator != 0 {
			return numerator / denominator
		}
	}
	if parsed, err := strconv.ParseFloat(rate, 64); err == nil && parsed > 0 {
		return parsed
	}
	return 30
}

func normalizedCaptionText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func resolveProjectPath(project, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(project, path)
}

func readSpeakerPeople(project, ref string) map[string]speakerPerson {
	if ref == "" {
		ref = filepath.Join("calibration", "speaker-map.json")
	}
	path := ref
	if !filepath.IsAbs(path) {
		path = filepath.Join(project, path)
	}
	var raw struct {
		People map[string]struct {
			DisplayName string `json:"display_name"`
			Title       string `json:"title"`
		} `json:"people"`
		Map map[string]string `json:"map"`
	}
	if err := readJSON(path, &raw); err != nil {
		return map[string]speakerPerson{}
	}
	out := map[string]speakerPerson{}
	for label, person := range raw.People {
		out[label] = speakerPerson{DisplayName: person.DisplayName, Title: person.Title}
	}
	for label := range raw.Map {
		if _, ok := out[label]; !ok {
			out[label] = speakerPerson{}
		}
	}
	return out
}

func readPresetIDs(project string) map[string]bool {
	var raw struct {
		Presets []struct {
			ID string `json:"id"`
		} `json:"presets"`
	}
	if err := readJSON(filepath.Join(project, "calibration", "framing-presets.json"), &raw); err != nil {
		return map[string]bool{}
	}
	out := map[string]bool{}
	for _, preset := range raw.Presets {
		out[preset.ID] = true
	}
	return out
}

func contractSchema(task string) string {
	switch task {
	case "captions":
		return "caption-cues.schema.json"
	case "audio":
		return "audio-intent.schema.json"
	case "supers":
		return "super-cards.schema.json"
	case "motion":
		return "motion-ramp.schema.json"
	case "sfx":
		return "sfx-cues.schema.json"
	case "broll":
		return "broll-plan.schema.json"
	default:
		return task + ".schema.json"
	}
}

func contractExample(task string) any {
	switch task {
	case "captions":
		return map[string]any{"version": "vflow-caption-cues/v1", "words_ref": "transcript/words.json", "style_id": "caption.default", "filler_clean": true, "cues": []map[string]any{{"id": "cap_001", "word_ids": []string{"w1"}, "start_frame": 0, "end_frame": 30}}}
	case "audio":
		return map[string]any{"version": "vflow-audio-intent/v1", "bed_ref": "media/music.wav", "duck_target_db": -18, "loudness_target_lufs": -16, "speech_segments": []map[string]any{{"id": "seg_001", "start_frame": 0, "end_frame": 120}}}
	case "motion":
		return map[string]any{"version": "vflow-motion-ramp/v1", "from_preset_id": "wide", "to_preset_id": "speaker_a_close", "ease": "linear", "start_frame": 0, "end_frame": 120}
	default:
		return map[string]any{"version": "vflow-" + task + "/v1", "brand_ref": "brand.json"}
	}
}

func renderInvocation(task, project string, adapter Adapter) string {
	if project == "" {
		project = "."
	}
	switch task {
	case "captions":
		return "vflow verify captions --project " + project + " --spec decisions/caption-cues.json --output artifacts/captions.srt --format json"
	case "audio":
		return "vflow verify audio --project " + project + " --spec decisions/audio-intent.json --output reports/audio-report.json --format json"
	case "supers":
		return adapter.Invocation + " && vflow verify supers --project " + project + " --spec decisions/super-cards.json --format json"
	case "motion":
		return adapter.Invocation + " && vflow verify motion --project " + project + " --spec decisions/motion-ramp.json --output reports/motion-diff.json --format json"
	default:
		return adapter.Invocation + " && vflow verify " + task + " --project " + project + " --spec decisions/" + task + ".json --format json"
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func containsText(text, expected string) bool {
	normalizedText := strings.ToLower(strings.Join(strings.Fields(text), " "))
	normalizedExpected := strings.ToLower(strings.Join(strings.Fields(expected), " "))
	return normalizedExpected == "" || strings.Contains(normalizedText, normalizedExpected)
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
