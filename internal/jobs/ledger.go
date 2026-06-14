package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Record struct {
	JobID         string    `json:"job_id"`
	Command       string    `json:"command"`
	Status        string    `json:"status"`
	Project       string    `json:"project"`
	StartedAt     time.Time `json:"started_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Retryable     bool      `json:"retryable"`
	ResumeCommand string    `json:"resume_command"`
	Path          string    `json:"path,omitempty"`
}

func NewRecord(project, command, status string, retryable bool) Record {
	now := time.Now().UTC()
	id := fmt.Sprintf("job_%d", now.UnixNano())
	return Record{
		JobID:         id,
		Command:       command,
		Status:        status,
		Project:       project,
		StartedAt:     now,
		UpdatedAt:     now,
		Retryable:     retryable,
		ResumeCommand: "vflow jobs resume " + id,
	}
}

func Write(project string, record Record) error {
	if record.JobID == "" {
		record = NewRecord(project, record.Command, record.Status, record.Retryable)
	}
	if record.Project == "" {
		record.Project = project
	}
	path := filepath.Join(project, "jobs", record.JobID+".json")
	record.Path = path
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func List(project string) ([]Record, error) {
	matches, err := filepath.Glob(filepath.Join(project, "jobs", "*.json"))
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(matches))
	for _, match := range matches {
		record, err := read(match)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	return records, nil
}

func Get(project, id string) (Record, error) {
	return read(filepath.Join(project, "jobs", id+".json"))
}

func read(path string) (Record, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return Record{}, err
	}
	record.Path = path
	return record, nil
}
