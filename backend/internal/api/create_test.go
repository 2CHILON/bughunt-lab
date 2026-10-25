package api

import (
	"net/http"
	"strings"
	"testing"
)

func post(t *testing.T, body string) (int, string, http.Header) {
	t.Helper()
	_, h := newTestServer("secret")
	rec := do(h, "POST", "/api/tasks", body, "Bearer secret")
	res := rec.Result()
	return res.StatusCode, rec.Body.String(), res.Header
}

func TestCreateRejectsBadInput(t *testing.T) {
	defer guard(t)
	cases := []struct{ name, body string }{
		{"invalid json", `{nope`},
		{"empty body", ``},
		{"blank title", `{"title":"   "}`},
		{"title too long", `{"title":"` + strings.Repeat("a", 201) + `"}`},
		{"unknown status", `{"title":"x","status":"banana"}`},
		{"priority out of range", `{"title":"x","priority":99}`},
		{"priority sent as string", `{"title":"x","priority":"3"}`},
		{"bad due date", `{"title":"x","dueDate":"tomorrow"}`},
		{"unknown field (typo)", `{"titel":"x"}`},
		{"two objects", `{"title":"a"}{"title":"b"}`},
		{"too many tags", `{"title":"x","tags":["1","2","3","4","5","6","7","8","9","10","11"]}`},
	}
	for _, c := range cases {
		code, body, hdr := post(t, c.body)
		if code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body %s)", c.name, code, body)
		}
		if ct := hdr.Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s: error Content-Type = %q", c.name, ct)
		}
	}
}

func TestCreateRejectsOversizedBody(t *testing.T) {
	defer guard(t)
	code, _, _ := post(t, `{"title":"`+strings.Repeat("a", 2<<20)+`"}`)
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", code)
	}
}

func TestCreateAppliesDefaultsAndNormalisesTags(t *testing.T) {
	defer guard(t)
	code, body, hdr := post(t, `{"title":"  hello  ","tags":["a"," b ",""]}`)
	if code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", code, body)
	}
	for _, want := range []string{`"title":"hello"`, `"priority":3`, `"status":"todo"`, `"tags":["a","b"]`} {
		if !strings.Contains(body, want) {
			t.Errorf("response missing %s: %s", want, body)
		}
	}
	if loc := hdr.Get("Location"); !strings.HasPrefix(loc, "/api/tasks/") {
		t.Errorf("Location = %q", loc)
	}
}