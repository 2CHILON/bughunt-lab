package store

import (
	"encoding/csv"
	"io"
	"strings"
	"sync"
	"testing"
)

func exportRows(t *testing.T, s *Store) [][]string {
	t.Helper()
	var b strings.Builder
	if err := s.ExportCSV(&b); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v\n%s", err, b.String())
	}
	return rows
}

func TestExportCSVQuotesSpecialCharacters(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: `fix "login", then deploy`, Priority: 2})
	rows := exportRows(t, s)
	if len(rows) != 2 || len(rows[1]) != 5 {
		t.Fatalf("unexpected shape: %v", rows)
	}
	if rows[1][1] != `fix "login", then deploy` {
		t.Fatalf("title came back as %q", rows[1][1])
	}
}

func TestExportCSVBlocksFormulaInjection(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: "=1+1"})
	s.Create(&Task{Title: "+1 and @mentions"})
	s.Create(&Task{Title: "normal"})
	rows := exportRows(t, s)
	for i, want := range []bool{true, true, false} {
		got := strings.HasPrefix(rows[i+1][1], "'")
		if got != want {
			t.Errorf("row %d title %q: neutralised=%v, want %v", i+1, rows[i+1][1], got, want)
		}
	}
}

func TestExportCSVSkipsDeletedTasks(t *testing.T) {
	defer guard(t)
	s := newSeeded(10)
	s.Delete(3)
	rows := exportRows(t, s)
	if len(rows) != 10 { // header + 9 tasks
		t.Fatalf("got %d rows, want 10", len(rows))
	}
	for _, r := range rows[1:] {
		if r[0] == "3" {
			t.Fatal("deleted task 3 was exported")
		}
	}
}

func TestExportCSVConcurrentWithWrites(t *testing.T) {
	defer guard(t)
	s := newSeeded(100)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.Create(&Task{Title: "w"})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.ExportCSV(io.Discard)
		}
	}()
	wg.Wait()
}