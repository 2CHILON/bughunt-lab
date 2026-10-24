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