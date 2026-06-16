package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitCreatesExpectedLayout(t *testing.T) {
	dir := t.TempDir()
	res, err := Init(dir, "panel_test", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "created" {
		t.Fatalf("expected created, got %s", res.Status)
	}
	for _, rel := range ExpectedLayout() {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("expected %s: %v", rel, err)
		}
	}
	res, err = Init(dir, "panel_test", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "no_change" {
		t.Fatalf("expected no_change, got %s", res.Status)
	}
}

func TestInitDryRunDoesNotCreateFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	res, err := Init(dir, "demo", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "planned" {
		t.Fatalf("expected planned, got %s", res.Status)
	}
	if _, err := os.Stat(filepath.Join(dir, "project.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create project.json: %v", err)
	}
}

func TestInitRejectsInvalidProjectID(t *testing.T) {
	_, err := Init(t.TempDir(), "bad id", false)
	if err == nil || !strings.Contains(err.Error(), "project id must start") {
		t.Fatalf("expected project id validation error, got %v", err)
	}
}

func TestLoadRejectsInvalidProjectContract(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.json"), []byte(`{"version":"vflow-project/v1","id":"bad id","root":"`+dir+`","created_at":"2026-06-14T00:00:00Z","updated_at":"2026-06-14T00:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "project id must start") {
		t.Fatalf("expected project id validation error, got %v", err)
	}
}

func TestValidateRejectsUpdatedBeforeCreated(t *testing.T) {
	err := Validate(Project{
		Version:   "vflow-project/v1",
		ID:        "valid_id",
		Root:      t.TempDir(),
		CreatedAt: time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 6, 14, 11, 0, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "updated_at must be at or after created_at") {
		t.Fatalf("expected timestamp validation error, got %v", err)
	}
}
