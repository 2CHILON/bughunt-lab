package store

import "testing"

func TestCreateManyAssignsConsecutiveIDsAndCopiesInput(t *testing.T) {
	defer guard(t)
	s := New("")
	s.Create(&Task{Title: "existing"})
	in := []Task{{Title: "a", Tags: []string{"x"}}, {Title: "b"}}
	out := s.CreateMany(in)
	if len(out) != 2 || out[0].ID != 2 || out[1].ID != 3 {
		t.Fatalf("unexpected result: %+v", out)
	}
	in[0].Title = "mutated"
	in[0].Tags[0] = "mutated"
	got := s.Get(2)
	if got.Title != "a" || got.Tags[0] != "x" {
		t.Fatalf("store changed behind its back: %+v", got)
	}
	if s.Len() != 3 {
		t.Fatalf("Len = %d, want 3", s.Len())
	}
}