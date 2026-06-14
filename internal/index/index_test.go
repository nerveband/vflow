package index

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexProjectWritesSQLiteFTSAndProvenance(t *testing.T) {
	projectPath := writeIndexFixture(t)
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")

	result, err := IndexProject(context.Background(), Options{
		ProjectPath: projectPath,
		DBPath:      dbPath,
		Commit:      true,
	})
	if err != nil {
		t.Fatalf("IndexProject returned error: %v", err)
	}
	if result.Status != "written" || result.DatabasePath != filepath.ToSlash(dbPath) {
		t.Fatalf("unexpected index result: %+v", result)
	}
	if result.ProjectCount != 1 || result.TranscriptWordCount != 4 {
		t.Fatalf("unexpected indexed counts: %+v", result)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected sqlite index: %v", err)
	}
	provenancePath := filepath.Join(projectPath, "reports", "provenance.json")
	if _, err := os.Stat(provenancePath); err != nil {
		t.Fatalf("expected provenance artifact: %v", err)
	}
	raw, err := os.ReadFile(provenancePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"index_database"`) || !strings.Contains(string(raw), `"transcript_words": 4`) {
		t.Fatalf("provenance missing index metadata: %s", raw)
	}

	search, err := SearchTranscripts(context.Background(), SearchOptions{
		DBPath: dbPath,
		Query:  "zakat",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("SearchTranscripts returned error: %v", err)
	}
	if search.Count != 1 {
		t.Fatalf("expected one search hit, got %+v", search)
	}
	match := search.Matches[0]
	if match.ProjectID != "index_fixture" || match.WordID != "w_000003" || match.StartFrame != 24 || match.EndFrame != 36 {
		t.Fatalf("unexpected search match: %+v", match)
	}
	if !strings.Contains(strings.ToLower(match.Snippet), "zakat") {
		t.Fatalf("expected snippet to include query term: %+v", match)
	}
}

func TestIndexProjectDryRunDoesNotWriteSQLiteOrProvenance(t *testing.T) {
	projectPath := writeIndexFixture(t)
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")

	result, err := IndexProject(context.Background(), Options{
		ProjectPath: projectPath,
		DBPath:      dbPath,
		Commit:      false,
	})
	if err != nil {
		t.Fatalf("IndexProject returned error: %v", err)
	}
	if result.Status != "planned" {
		t.Fatalf("expected planned status, got %+v", result)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote sqlite DB: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectPath, "reports", "provenance.json")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote provenance: %v", err)
	}
}

func writeIndexFixture(t *testing.T) string {
	t.Helper()

	projectPath := t.TempDir()
	mustMkdir(t, filepath.Join(projectPath, "transcript"))
	mustMkdir(t, filepath.Join(projectPath, "reports"))
	mustWriteJSON(t, filepath.Join(projectPath, "project.json"), map[string]any{
		"version":    "vflow-project/v1",
		"id":         "index_fixture",
		"root":       projectPath,
		"created_at": "2026-06-14T00:00:00Z",
		"updated_at": "2026-06-14T00:00:00Z",
	})
	mustWriteJSON(t, filepath.Join(projectPath, "transcript", "words.json"), map[string]any{
		"version":         "vflow-words/v1",
		"source_media_id": "camera_a",
		"rate":            "30000/1001",
		"words": []map[string]any{
			{"id": "w_000001", "text": "we", "speaker_label": "SPEAKER_00", "start_frame": 0, "end_frame": 12, "confidence": 1, "provider": "fixture"},
			{"id": "w_000002", "text": "collect", "speaker_label": "SPEAKER_00", "start_frame": 12, "end_frame": 24, "confidence": 1, "provider": "fixture"},
			{"id": "w_000003", "text": "zakat", "speaker_label": "SPEAKER_00", "start_frame": 24, "end_frame": 36, "confidence": 1, "provider": "fixture"},
			{"id": "w_000004", "text": "wisely", "speaker_label": "SPEAKER_00", "start_frame": 36, "end_frame": 48, "confidence": 1, "provider": "fixture"},
		},
	})
	return projectPath
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
