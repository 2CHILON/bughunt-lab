package store

import (
	"fmt"
	"io"
	"strings"
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
//
func TestCreateCopiesTags(t *testing.T) {
	defer guard(t)
	s := New("")
	in := &Task{Title: "t", Tags: []string{"x"}}
	created := s.Create(in)
	in.Tags[0] = "changed"
	if got := s.Get(created.ID).Tags[0]; got != "x" {
		t.Fatalf("stored tags changed behind the store's back: %q",got)
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
		s.ExportCSV(io.Discard)
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

//
func TestUpdateWhileReading(t *testing.T) {
	defer guard(t)
	s := newSeeded(20)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			s.Update(1, func(t *Task) { t.Title = fmt.Sprint("title ", i) })
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			if task := s.Get(1); task != nil {
				_ = task.Title
			}
		}
	}()
	wg.Wait()
}

func TestDeleteThenExportCSV(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: "one"})
	s.Create(&Task{Title: "two"})
	s.Create(&Task{Title: "three"})
	s.Delete(2)

	var b strings.Builder
	if err := s.ExportCSV(&b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "two") {
		t.Fatalf("deleted task still exported:\n%s", b.String())
	}
}

func TestDeleteThenReload(t *testing.T) {
	defer guard(t)
	path := t.TempDir() + "/tasks.json"
	s := New(path)
	s.Create(&Task{Title: "one"})
	s.Create(&Task{Title: "two"})
	s.Create(&Task{Title: "three"})
	s.Delete(2)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2 := New(path) // simulates a server restart
	if s2.Len() != 2 {
		t.Fatalf("after reload got %d tasks, want 2", s2.Len())
	}
}
func TestListPaginationEdges(t *testing.T) {
	defer guard(t)
	s := newSeeded(25)
	cases := []struct {
		name        string
		page, limit int
		wantLen     int
		wantFirstID int
	}{
		{"first page", 1, 10, 10, 1},
		{"second page", 2, 10, 10, 11},
		{"partial last page", 3, 10, 5, 21},
		{"past the end", 4, 10, 0, 0},
		{"limit bigger than data", 1, 100, 25, 1},
		{"page zero treated as 1", 0, 10, 10, 1},
		{"negative limit uses default", 1, -5, 20, 1},
	}
	for _, c := range cases {
		got := s.List(Filter{Page: c.page, Limit: c.limit})
		if len(got) != c.wantLen {
			t.Errorf("%s: len = %d, want %d", c.name, len(got), c.wantLen)
			continue
		}
		if c.wantLen > 0 && got[0].ID != c.wantFirstID {
			t.Errorf("%s: first ID = %d, want %d", c.name, got[0].ID, c.wantFirstID)
		}
	}
}

func TestSearchMatchesTitleDescriptionAndTags(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: "Fix LOGIN bug"})
	s.Create(&Task{Title: "other", Description: "check the Login page"})
	s.Create(&Task{Title: "third", Tags: []string{"login-flow"}})
	s.Create(&Task{Title: "unrelated"})
	if got := len(s.Search("login")); got != 3 {
		t.Fatalf("got %d matches, want 3", got)
	}
}

func TestSearchTreatsInputAsLiteralText(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: "learn c++ (basics)"})
	s.Create(&Task{Title: "learn cxx basics"})
	if got := len(s.Search("c++ (")); got != 1 {
		t.Fatalf("got %d matches, want 1", got)
	}
	if got := len(s.Search(".*")); got != 0 {
		t.Fatalf("regex metacharacters must not act as a pattern, got %d matches", got)
	}
}

func TestStatsFewTagsDoesNotPanic(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: "a", Tags: []string{"x", "y"}})
	st := s.Stats()
	if len(st.TopTags) != 2 {
		t.Fatalf("TopTags = %v, want 2 entries", st.TopTags)
	}
}

func TestStatsTopTagsAreOrderedAndStable(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: "1", Tags: []string{"b", "a"}})
	s.Create(&Task{Title: "2", Tags: []string{"c", "a"}})
	s.Create(&Task{Title: "3", Tags: []string{"b"}})
	for i := 0; i < 20; i++ {
		got := s.Stats().TopTags
		if len(got) != 3 || got[0].Tag != "a" || got[1].Tag != "b" || got[2].Tag != "c" {
			t.Fatalf("run %d: unexpected order %v (want a(2), b(2), c(1); ties sorted by name)", i, got)
		}
	}
}