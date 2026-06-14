package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeliverFileRejectsExistingWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.json")
	dst := filepath.Join(dir, "delivered.json")
	if err := os.WriteFile(src, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := DeliverFile(src, dst, false)
	if err == nil {
		t.Fatalf("expected existing destination to be rejected")
	}
	got, _ := os.ReadFile(dst)
	if string(got) != `{"old":true}` {
		t.Fatalf("destination changed without overwrite: %s", got)
	}
}

func TestDeliverFileWritesAtomicallyWithOverwrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.json")
	dst := filepath.Join(dir, "nested", "delivered.json")
	if err := os.WriteFile(src, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := DeliverFile(src, dst, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "delivered" || res.Output != dst {
		t.Fatalf("unexpected result: %+v", res)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("unexpected destination content: %s", got)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(dst), ".vflow-deliver-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}
