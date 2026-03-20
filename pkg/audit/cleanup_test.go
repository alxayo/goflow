package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyRetention_DeletesOldest(t *testing.T) {
	tmp := t.TempDir()
	// Create 5 "run" directories with sortable names.
	names := []string{
		"2026-03-20T10-00-00_run",
		"2026-03-20T11-00-00_run",
		"2026-03-20T12-00-00_run",
		"2026-03-20T13-00-00_run",
		"2026-03-20T14-00-00_run",
	}
	for _, n := range names {
		if err := os.Mkdir(filepath.Join(tmp, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := ApplyRetention(tmp, 3); err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}

	entries, _ := os.ReadDir(tmp)
	if len(entries) != 3 {
		t.Fatalf("expected 3 dirs, got %d", len(entries))
	}

	// The 3 newest should remain.
	kept := map[string]bool{}
	for _, e := range entries {
		kept[e.Name()] = true
	}
	for _, want := range names[2:] {
		if !kept[want] {
			t.Errorf("expected %s to be kept", want)
		}
	}
	for _, gone := range names[:2] {
		if kept[gone] {
			t.Errorf("expected %s to be deleted", gone)
		}
	}
}

func TestApplyRetention_FewerThanRetention(t *testing.T) {
	tmp := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		os.Mkdir(filepath.Join(tmp, n), 0o755)
	}

	if err := ApplyRetention(tmp, 5); err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}

	entries, _ := os.ReadDir(tmp)
	if len(entries) != 3 {
		t.Errorf("expected 3 dirs, got %d", len(entries))
	}
}

func TestApplyRetention_ZeroKeepsAll(t *testing.T) {
	tmp := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		os.Mkdir(filepath.Join(tmp, n), 0o755)
	}

	if err := ApplyRetention(tmp, 0); err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}

	entries, _ := os.ReadDir(tmp)
	if len(entries) != 3 {
		t.Errorf("expected 3 dirs, got %d", len(entries))
	}
}

func TestApplyRetention_NegativeKeepsAll(t *testing.T) {
	tmp := t.TempDir()
	for _, n := range []string{"a", "b"} {
		os.Mkdir(filepath.Join(tmp, n), 0o755)
	}

	if err := ApplyRetention(tmp, -1); err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}

	entries, _ := os.ReadDir(tmp)
	if len(entries) != 2 {
		t.Errorf("expected 2 dirs, got %d", len(entries))
	}
}

func TestApplyRetention_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	if err := ApplyRetention(tmp, 3); err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}
}

func TestApplyRetention_MissingDir(t *testing.T) {
	if err := ApplyRetention("/tmp/nonexistent-audit-dir-xyz", 3); err != nil {
		t.Fatalf("ApplyRetention on missing dir should be no-op, got: %v", err)
	}
}

func TestApplyRetention_SkipsNonDirectories(t *testing.T) {
	tmp := t.TempDir()
	// Create 3 dirs and 1 file.
	for _, n := range []string{"2026-01", "2026-02", "2026-03"} {
		os.Mkdir(filepath.Join(tmp, n), 0o755)
	}
	os.WriteFile(filepath.Join(tmp, "notes.txt"), []byte("hi"), 0o644)

	if err := ApplyRetention(tmp, 2); err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}

	entries, _ := os.ReadDir(tmp)
	// Should have 2 dirs + 1 file = 3 entries.
	dirCount := 0
	fileCount := 0
	for _, e := range entries {
		if e.IsDir() {
			dirCount++
		} else {
			fileCount++
		}
	}
	if dirCount != 2 {
		t.Errorf("expected 2 dirs, got %d", dirCount)
	}
	if fileCount != 1 {
		t.Errorf("expected 1 file, got %d", fileCount)
	}
}
