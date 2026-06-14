package project

import (
	"os"
	"path/filepath"
	"testing"
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
