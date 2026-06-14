package index

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	vproject "github.com/nerveband/vflow/internal/project"
	vtranscript "github.com/nerveband/vflow/internal/transcript"

	_ "modernc.org/sqlite"
)

const (
	ProjectIndexVersion = "vflow-project-index/v1"
	ProvenanceVersion   = "vflow-provenance/v1"
	SearchVersion       = "vflow-transcript-search/v1"
)

type Options struct {
	ProjectPath string
	DBPath      string
	Commit      bool
}

type Result struct {
	Version             string   `json:"version"`
	Status              string   `json:"status"`
	ProjectPath         string   `json:"project_path"`
	ProjectID           string   `json:"project_id"`
	DatabasePath        string   `json:"database_path"`
	ProvenancePath      string   `json:"provenance_path"`
	ProjectCount        int      `json:"project_count"`
	SourceCount         int      `json:"source_count"`
	TranscriptWordCount int      `json:"transcript_word_count"`
	ArtifactCount       int      `json:"artifact_count"`
	ProviderRunCount    int      `json:"provider_run_count"`
	NLEEventCount       int      `json:"nle_event_count"`
	ArtifactPaths       []string `json:"artifact_paths,omitempty"`
}

type SearchOptions struct {
	ProjectPath string
	DBPath      string
	Query       string
	Limit       int
}

type SearchResult struct {
	Version    string            `json:"version"`
	Query      string            `json:"query"`
	DataSource string            `json:"data_source"`
	Count      int               `json:"count"`
	Matches    []TranscriptMatch `json:"matches"`
}

type TranscriptMatch struct {
	ProjectID    string `json:"project_id"`
	ProjectPath  string `json:"project_path"`
	WordID       string `json:"word_id"`
	Text         string `json:"text"`
	SpeakerLabel string `json:"speaker_label,omitempty"`
	StartFrame   int64  `json:"start_frame"`
	EndFrame     int64  `json:"end_frame"`
	SourceMedia  string `json:"source_media_id,omitempty"`
	Snippet      string `json:"snippet"`
}

type sourceRecord struct {
	Source          string  `json:"source"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	DurationSeconds float64 `json:"duration_seconds"`
	FrameRate       string  `json:"frame_rate"`
	Codec           string  `json:"codec"`
}

func IndexProject(ctx context.Context, opts Options) (Result, error) {
	if opts.ProjectPath == "" {
		opts.ProjectPath = "."
	}
	absProjectPath, err := filepath.Abs(opts.ProjectPath)
	if err != nil {
		return Result{}, err
	}
	project, err := vproject.Load(absProjectPath)
	if err != nil {
		return Result{}, err
	}
	if project.ID == "" {
		project.ID = filepath.Base(absProjectPath)
	}
	dbPath, err := resolveDBPath(opts.DBPath)
	if err != nil {
		return Result{}, err
	}
	provenancePath := filepath.Join(absProjectPath, "reports", "provenance.json")

	words, _ := vtranscript.ReadWords(absProjectPath)
	sources := readSourceReview(absProjectPath)
	artifacts := discoverArtifacts(absProjectPath)
	providerRuns := filterArtifacts(artifacts, "jobs/")
	nleEvents := filterArtifacts(artifacts, "exports/", "imports/")

	result := Result{
		Version:             ProjectIndexVersion,
		Status:              "planned",
		ProjectPath:         filepath.ToSlash(absProjectPath),
		ProjectID:           project.ID,
		DatabasePath:        filepath.ToSlash(dbPath),
		ProvenancePath:      filepath.ToSlash(provenancePath),
		ProjectCount:        1,
		SourceCount:         len(sources),
		TranscriptWordCount: len(words.Words),
		ArtifactCount:       len(artifacts),
		ProviderRunCount:    len(providerRuns),
		NLEEventCount:       len(nleEvents),
		ArtifactPaths:       artifacts,
	}
	if !opts.Commit {
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return Result{}, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return Result{}, err
	}
	defer db.Close()
	if err := migrate(ctx, db); err != nil {
		return Result{}, err
	}
	if err := writeProjectIndex(ctx, db, project, absProjectPath, words, sources, artifacts, providerRuns, nleEvents); err != nil {
		return Result{}, err
	}
	if err := writeProvenance(provenancePath, result); err != nil {
		return Result{}, err
	}
	result.Status = "written"
	return result, nil
}

func SearchTranscripts(ctx context.Context, opts SearchOptions) (SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	dbPath, err := resolveDBPath(opts.DBPath)
	if err != nil {
		return SearchResult{}, err
	}
	projectID := ""
	if opts.ProjectPath != "" {
		if project, err := vproject.Load(opts.ProjectPath); err == nil {
			projectID = project.ID
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return SearchResult{}, err
	}
	defer db.Close()

	ftsQuery := buildFTSQuery(opts.Query)
	rows, err := db.QueryContext(ctx, `
SELECT project_id, project_path, word_id, text, speaker_label, start_frame, end_frame, source_media_id
FROM transcript_fts
WHERE transcript_fts MATCH ?
  AND (? = '' OR project_id = ?)
ORDER BY rank
LIMIT ?`, ftsQuery, projectID, projectID, opts.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	defer rows.Close()

	result := SearchResult{Version: SearchVersion, Query: opts.Query, DataSource: "local", Matches: []TranscriptMatch{}}
	for rows.Next() {
		var match TranscriptMatch
		if err := rows.Scan(&match.ProjectID, &match.ProjectPath, &match.WordID, &match.Text, &match.SpeakerLabel, &match.StartFrame, &match.EndFrame, &match.SourceMedia); err != nil {
			return SearchResult{}, err
		}
		match.Snippet = match.Text
		result.Matches = append(result.Matches, match)
	}
	if err := rows.Err(); err != nil {
		return SearchResult{}, err
	}
	result.Count = len(result.Matches)
	return result, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			root TEXT NOT NULL,
			indexed_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sources (
			project_id TEXT NOT NULL,
			source TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			width INTEGER,
			height INTEGER,
			duration_seconds REAL,
			frame_rate TEXT,
			codec TEXT,
			PRIMARY KEY(project_id, source)
		)`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			project_id TEXT NOT NULL,
			path TEXT NOT NULL,
			kind TEXT NOT NULL,
			PRIMARY KEY(project_id, path)
		)`,
		`CREATE TABLE IF NOT EXISTS provider_runs (
			project_id TEXT NOT NULL,
			artifact_path TEXT NOT NULL,
			status TEXT,
			PRIMARY KEY(project_id, artifact_path)
		)`,
		`CREATE TABLE IF NOT EXISTS nle_events (
			project_id TEXT NOT NULL,
			artifact_path TEXT NOT NULL,
			event_type TEXT NOT NULL,
			PRIMARY KEY(project_id, artifact_path)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS transcript_fts USING fts5(
			project_id UNINDEXED,
			project_path UNINDEXED,
			word_id UNINDEXED,
			text,
			speaker_label UNINDEXED,
			start_frame UNINDEXED,
			end_frame UNINDEXED,
			source_media_id UNINDEXED,
			rate UNINDEXED,
			tokenize='unicode61'
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func writeProjectIndex(ctx context.Context, db *sql.DB, project vproject.Project, projectPath string, words vtranscript.Words, sources []sourceRecord, artifacts, providerRuns, nleEvents []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `INSERT INTO projects(id, path, root, indexed_at) VALUES(?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET path=excluded.path, root=excluded.root, indexed_at=excluded.indexed_at`,
		project.ID, filepath.ToSlash(projectPath), filepath.ToSlash(project.Root), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	for _, table := range []string{"sources", "artifacts", "provider_runs", "nle_events"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE project_id = ?", project.ID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM transcript_fts WHERE project_id = ?`, project.ID); err != nil {
		return err
	}
	for _, source := range sources {
		if _, err := tx.ExecContext(ctx, `INSERT INTO sources(project_id, source, fingerprint, width, height, duration_seconds, frame_rate, codec) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			project.ID, source.Source, sourceFingerprint(source), source.Width, source.Height, source.DurationSeconds, source.FrameRate, source.Codec); err != nil {
			return err
		}
	}
	for _, artifact := range artifacts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO artifacts(project_id, path, kind) VALUES(?, ?, ?)`, project.ID, artifact, artifactKind(artifact)); err != nil {
			return err
		}
	}
	for _, artifact := range providerRuns {
		if _, err := tx.ExecContext(ctx, `INSERT INTO provider_runs(project_id, artifact_path, status) VALUES(?, ?, ?)`, project.ID, artifact, "recorded"); err != nil {
			return err
		}
	}
	for _, artifact := range nleEvents {
		if _, err := tx.ExecContext(ctx, `INSERT INTO nle_events(project_id, artifact_path, event_type) VALUES(?, ?, ?)`, project.ID, artifact, nleEventType(artifact)); err != nil {
			return err
		}
	}
	for _, word := range words.Words {
		if _, err := tx.ExecContext(ctx, `INSERT INTO transcript_fts(project_id, project_path, word_id, text, speaker_label, start_frame, end_frame, source_media_id, rate) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			project.ID, filepath.ToSlash(projectPath), word.ID, word.Text, word.SpeakerLabel, word.StartFrame, word.EndFrame, words.SourceMediaID, words.Rate); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func writeProvenance(path string, result Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	provenance := map[string]any{
		"version":        ProvenanceVersion,
		"project_id":     result.ProjectID,
		"project_path":   result.ProjectPath,
		"index_database": result.DatabasePath,
		"indexed_at":     time.Now().UTC().Format(time.RFC3339),
		"counts": map[string]int{
			"projects":         result.ProjectCount,
			"sources":          result.SourceCount,
			"transcript_words": result.TranscriptWordCount,
			"artifacts":        result.ArtifactCount,
			"provider_runs":    result.ProviderRunCount,
			"nle_events":       result.NLEEventCount,
		},
		"artifact_paths": result.ArtifactPaths,
	}
	raw, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func resolveDBPath(path string) (string, error) {
	if path == "" {
		path = os.Getenv("VFLOW_INDEX_PATH")
	}
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, ".vflow", "index.sqlite")
	}
	return filepath.Abs(path)
}

func readSourceReview(projectPath string) []sourceRecord {
	raw, err := os.ReadFile(filepath.Join(projectPath, "source-media-review.json"))
	if err != nil {
		return nil
	}
	var envelope struct {
		Sources []sourceRecord `json:"sources"`
		Source  sourceRecord   `json:"source"`
		Version string         `json:"version"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	out := append([]sourceRecord{}, envelope.Sources...)
	if envelope.Source.Source != "" {
		out = append(out, envelope.Source)
	}
	if len(out) == 0 && envelope.Version != "" {
		var direct sourceRecord
		if err := json.Unmarshal(raw, &direct); err == nil && direct.Source != "" {
			out = append(out, direct)
		}
	}
	return out
}

func discoverArtifacts(projectPath string) []string {
	roots := []string{"transcript", "decisions", "timeline", "exports", "imports", "reports", "renders", "review", "calibration", "jobs"}
	artifacts := []string{}
	for _, root := range roots {
		base := filepath.Join(projectPath, root)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(projectPath, path)
			if err != nil {
				return nil
			}
			artifacts = append(artifacts, filepath.ToSlash(rel))
			return nil
		})
	}
	return artifacts
}

func filterArtifacts(artifacts []string, prefixes ...string) []string {
	var out []string
	for _, artifact := range artifacts {
		for _, prefix := range prefixes {
			if strings.HasPrefix(artifact, prefix) {
				out = append(out, artifact)
				break
			}
		}
	}
	return out
}

func sourceFingerprint(source sourceRecord) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%.6f|%s|%s", source.Source, source.Width, source.Height, source.DurationSeconds, source.FrameRate, source.Codec)))
	return hex.EncodeToString(h[:])
}

func artifactKind(path string) string {
	if before, _, ok := strings.Cut(path, "/"); ok {
		return before
	}
	return "artifact"
}

func nleEventType(path string) string {
	switch {
	case strings.HasPrefix(path, "exports/"):
		return "export"
	case strings.HasPrefix(path, "imports/"):
		return "import"
	default:
		return "artifact"
	}
}

func buildFTSQuery(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return `""`
	}
	escaped := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.ReplaceAll(field, `"`, `""`)
		escaped = append(escaped, `"`+field+`"`)
	}
	return strings.Join(escaped, " ")
}
