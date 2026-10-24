package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlushPersistsAndReloads(t *testing.T) {
	defer guard(t)
	path := filepath.Join(t.TempDir(), "tasks.json")
	s := New(path)
	s.Create(&Task{Title: "one"})
	s.Create(&Task{Title: "two"})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2 := New(path)
	defer s2.Close()
	if s2.Len() != 2 {
		t.Fatalf("reloaded %d tasks, want 2", s2.Len())
	}
}

func TestFlushLeavesNoTempFiles(t *testing.T) {
	defer guard(t)
	dir := t.TempDir()
	s := New(filepath.Join(dir, "tasks.json"))
	for i := 0; i < 20; i++ {
		s.Create(&Task{Title: "x"})
		s.Flush()
	}
	s.Close()
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "tasks.json" {
		t.Fatalf("unexpected files in data dir: %v", entries)
	}
}

func TestFlushErrorIsReportedAndRetried(t *testing.T) {
	defer guard(t)
	dir := t.TempDir()
	sub := filepath.Join(dir, "missing")
	s := New(filepath.Join(sub, "tasks.json"))
	defer s.Close()
	s.Create(&Task{Title: "x"})
	if err := s.Flush(); err == nil {
		t.Fatal("expected an error: directory does not exist")
	}
	os.Mkdir(sub, 0o755)
	if err := s.Flush(); err != nil {
		t.Fatalf("retry should succeed once the directory exists: %v", err)
	}
}

func BenchmarkFlush(b *testing.B) {
	s := New(filepath.Join(b.TempDir(), "tasks.json"))
	defer s.Close()
	s.Seed(2000)
	s.Flush()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Create(&Task{Title: "x"})
		s.Flush()
	}
}

func TestOpenMissingFileIsFreshStart(t *testing.T) {
	defer guard(t)
	s, err := Open(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatalf("a missing file must not be an error: %v", err)
	}
	defer s.Close()
	if s.Len() != 0 {
		t.Fatalf("Len = %d, want 0", s.Len())
	}
}

func TestOpenCorruptFileFailsAndLeavesFileUntouched(t *testing.T) {
	defer guard(t)
	path := filepath.Join(t.TempDir(), "tasks.json")
	corrupt := []byte(`[{"id":1,"title":"precious"},{"id":2,"ti`)
	os.WriteFile(path, corrupt, 0o644)
	s, err := Open(path)
	if err == nil {
		s.Close()
		t.Fatal("expected an error for a corrupt file")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(corrupt) {
		t.Fatalf("the corrupt file was modified: %q", got)
	}
}

func TestOpenSkipsNullAndDuplicateEntries(t *testing.T) {
	defer guard(t)
	path := filepath.Join(t.TempDir(), "tasks.json")
	os.WriteFile(path, []byte(`[null,{"id":1,"title":"a"},{"id":1,"title":"dup"},{"id":5,"title":"b"}]`), 0o644)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Len() != 2 {
		t.Fatalf("Len = %d, want 2", s.Len())
	}
	if got := s.Get(1); got == nil || got.Title != "a" {
		t.Fatalf("first copy of ID 1 should win, got %+v", got)
	}
	if c := s.Create(&Task{Title: "next"}); c.ID != 6 {
		t.Fatalf("next ID = %d, want 6", c.ID)
	}
}