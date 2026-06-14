package nle

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func Export(opts Options, segments []Segment) ExportResult {
	if opts.Target == "" {
		opts.Target = "sidecar"
	}
	return ExportResult{
		Target: opts.Target,
		Output: opts.Output,
		Sidecar: Sidecar{
			Version:  "vflow-nle-sidecar/v1",
			Target:   opts.Target,
			Segments: segments,
		},
	}
}

func WriteExport(projectPath string, res ExportResult) error {
	if res.Output != "" {
		if err := os.MkdirAll(filepath.Dir(res.Output), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(res.Output, []byte(exportText(res)+"\n"), 0o644); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(res.Sidecar, "", "  ")
	if err != nil {
		return err
	}
	sidecarPath := filepath.Join(projectPath, "exports", "sidecars", res.Target+"-vflow-sidecar.json")
	if err := os.MkdirAll(filepath.Dir(sidecarPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(sidecarPath, append(raw, '\n'), 0o644)
}

func exportText(res ExportResult) string {
	switch res.Target {
	case "edl":
		return "TITLE: vflow\nFCM: NON-DROP FRAME"
	case "fcpxml":
		return `<?xml version="1.0" encoding="UTF-8"?><fcpxml version="1.11"></fcpxml>`
	case "mlt":
		return `<mlt title="vflow"></mlt>`
	case "otio":
		return `{"OTIO_SCHEMA":"Timeline.1","name":"vflow"}`
	default:
		return "{}"
	}
}
