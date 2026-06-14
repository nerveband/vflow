package jobs

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWriteListAndGetJobs(t *testing.T) {
	project := t.TempDir()
	record := Record{
		JobID:         "job_test",
		Command:       "render preview",
		Status:        "succeeded",
		Project:       project,
		StartedAt:     time.Date(2026, 6, 14, 1, 2, 3, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 6, 14, 1, 2, 4, 0, time.UTC),
		Retryable:     true,
		ResumeCommand: "vflow jobs resume job_test",
	}
	if err := Write(project, record); err != nil {
		t.Fatal(err)
	}

	list, err := List(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].JobID != "job_test" {
		t.Fatalf("unexpected list: %+v", list)
	}
	got, err := Get(project, "job_test")
	if err != nil {
		t.Fatal(err)
	}
	if got.Command != "render preview" || got.Status != "succeeded" {
		t.Fatalf("unexpected job: %+v", got)
	}
	if got.Path != filepath.Join(project, "jobs", "job_test.json") {
		t.Fatalf("unexpected path: %s", got.Path)
	}
}

func TestNewRecordPopulatesResumeCommand(t *testing.T) {
	project := t.TempDir()
	record := NewRecord(project, "qa analyze", "failed", true)
	if record.JobID == "" || record.Project != project || record.ResumeCommand == "" {
		t.Fatalf("record missing generated fields: %+v", record)
	}
	if record.UpdatedAt.Before(record.StartedAt) {
		t.Fatalf("updated_at before started_at: %+v", record)
	}
}
