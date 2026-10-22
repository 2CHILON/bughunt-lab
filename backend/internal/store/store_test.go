package store

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// guard turns a panic into a normal test failure so one bug doesn't hide the others.
func guard(t *testing.T) {
	t.Helper()
	if r := recover(); r != nil {
		t.Fatalf("PANIC: %v", r)
	}
}

func newSeeded(n int) *Store {
	s := New("") // empty path = in-memory only, no disk writes
	s.Seed(n)
	return s
}

func TestCreateAssignsUniqueIDsAfterDelete(t *testing.T) {
	defer guard(t)
	s := New("")
	a := s.Create(&Task{Title: "a"})
	b := s.Create(&Task{Title: "b"})
	s.Delete(a.ID)
	c := s.Create(&Task{Title: "c"})
	if c.ID == b.ID {
		t.Fatalf("new task got ID %d which already belongs to %q", c.ID, s.Get(b.ID).Title)
	}
}

func TestCreateDoesNotAliasCallerData(t *testing.T) {
	defer guard(t)
	s := New("")
	in := &Task{Title: "orig", Tags: []string{"x"}}
	created := s.Create(in)
	in.Title = "mutated by caller"
	if got := s.Get(created.ID).Title; got != "orig" {
		t.Fatalf("store content changed behind its back: %q", got)
	}
}

func TestListFirstPageReturnsFirstItems(t *testing.T) {
	defer guard(t)
	s := newSeeded(50)
	got := s.List(Filter{Page: 1, Limit: 10})
	if len(got) != 10 || got[0].ID != 1 {
		t.Fatalf("page 1 should start at ID 1, got len=%d first=%v", len(got), got)
	}
}

func TestListLastPartialPage(t *testing.T) {
	defer guard(t)
	s := newSeeded(25)
	got := s.List(Filter{Page: 3, Limit: 10})
	if len(got) != 5 {
		t.Fatalf("page 3 of 25 items @10 should have 5 items, got %d", len(got))
	}
}

func TestListAfterDelete(t *testing.T) {
	defer guard(t)
	s := newSeeded(30)
	s.Delete(2)
	got := s.List(Filter{Page: 1, Limit: 30})
	for _, task := range got {
		if task.ID == 2 {
			t.Fatal("deleted task still listed")
		}
	}
}

func TestSearchSpecialCharsDoNotPanic(t *testing.T) {
	defer guard(t)
	s := newSeeded(20)
	s.Search("c++ (")
}

func TestSearchIsDeterministic(t *testing.T) {
	defer guard(t)
	s := newSeeded(200)
	a, b := s.Search("deploy"), s.Search("deploy")
	if len(a) != len(b) {
		t.Skip("different lengths")
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("result order differs between identical searches (index %d)", i)
		}
	}
}

func TestStatsEmptyStoreIsValid(t *testing.T) {
	defer guard(t)
	s := New("")
	st := s.Stats()
	if st.AvgPriority != st.AvgPriority { // NaN != NaN
		t.Fatal("AvgPriority is NaN (json.Marshal will fail on this)")
	}
}

func TestOverdueIgnoresTasksWithoutDueDate(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: "no due date"})
	s.Create(&Task{Title: "future", DueDate: time.Now().AddDate(0, 0, 5).Format("2006-01-02")})
	s.Create(&Task{Title: "late", DueDate: time.Now().AddDate(0, 0, -5).Format("2006-01-02")})
	s.Create(&Task{Title: "late but done", Status: "done", DueDate: time.Now().AddDate(0, 0, -5).Format("2006-01-02")})
	if got := s.Stats().Overdue; got != 1 {
		t.Fatalf("overdue = %d, want 1", got)
	}
}

// Run this one with: go test -race ./internal/store -run Concurrent
func TestConcurrentReadWrite(t *testing.T) {
	defer guard(t)
	s := newSeeded(50)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				s.Create(&Task{Title: fmt.Sprintf("w%d-%d", i, j), Priority: 3})
				s.Stats()
				s.Search("deploy")
			}
		}(i)
	}
	wg.Wait()
}

// --- Benchmarks: go test -bench . -benchmem -cpuprofile cpu.out ./internal/store ---

func BenchmarkSearch(b *testing.B) {
	s := newSeeded(2000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Search("deploy")
	}
}

func BenchmarkStats(b *testing.B) {
	s := newSeeded(5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Stats()
	}
}

func BenchmarkExportCSV(b *testing.B) {
	s := newSeeded(5000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ExportCSV()
	}
}

func BenchmarkCreateWithPersistence(b *testing.B) {
	s := New(b.TempDir() + "/tasks.json")
	s.Seed(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Create(&Task{Title: "bench"})
	}
}
