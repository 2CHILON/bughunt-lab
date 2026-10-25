package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func getTask(t *testing.T, h http.Handler, id string) map[string]any {
	t.Helper()
	rec := do(h, "GET", "/api/tasks/"+id, "", "")
	if rec.Code != 200 {
		t.Fatalf("GET %s = %d", id, rec.Code)
	}
	var m map[string]any
	json.Unmarshal(rec.Body.Bytes(), &m)
	return m
}

func TestPutReplacesTheTask(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	before := getTask(t, h, "5")
	body := `{"id":777,"title":"  renamed ","description":"d","status":"doing","priority":5,"tags":["x"," y",""],"dueDate":"2030-01-02"}`
	rec := do(h, "PUT", "/api/tasks/5", body, "Bearer secret")
	if rec.Code != 200 {
		t.Fatalf("PUT = %d (%s)", rec.Code, rec.Body.String())
	}
	after := getTask(t, h, "5")
	if after["id"] != float64(5) {
		t.Errorf("the URL's id must win over the body's id, got %v", after["id"])
	}
	if after["title"] != "renamed" || after["status"] != "doing" || after["priority"] != float64(5) {
		t.Errorf("fields not replaced: %v", after)
	}
	if tags, _ := json.Marshal(after["tags"]); string(tags) != `["x","y"]` {
		t.Errorf("tags = %s, want [\"x\",\"y\"]", tags)
	}
	if after["createdAt"] != before["createdAt"] {
		t.Errorf("createdAt changed: %v -> %v", before["createdAt"], after["createdAt"])
	}
}

func TestPutRejectsIncompleteOrInvalidBodiesAndLeavesTaskUntouched(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	before, _ := json.Marshal(getTask(t, h, "7"))
	cases := []struct{ name, body string }{
		{"only a title (would wipe the rest)", `{"title":"x"}`},
		{"blank title", `{"title":" ","status":"todo","priority":2}`},
		{"unknown status", `{"title":"x","status":"banana","priority":2}`},
		{"priority out of range", `{"title":"x","status":"todo","priority":9}`},
		{"bad due date", `{"title":"x","status":"todo","priority":2,"dueDate":"soon"}`},
		{"unknown field", `{"title":"x","status":"todo","priority":2,"titel":"typo"}`},
		{"invalid json", `{nope`},
		{"empty body", ``},
	}
	for _, c := range cases {
		rec := do(h, "PUT", "/api/tasks/7", c.body, "Bearer secret")
		if rec.Code != 400 {
			t.Errorf("%s: status = %d, want 400 (%s)", c.name, rec.Code, rec.Body.String())
		}
	}
	after, _ := json.Marshal(getTask(t, h, "7"))
	if string(before) != string(after) {
		t.Fatalf("a rejected PUT changed the task:\nbefore %s\nafter  %s", before, after)
	}
}

func TestPutRejectsOversizedBody(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	big := `{"title":"` + strings.Repeat("a", 2<<20) + `","status":"todo","priority":1}`
	if rec := do(h, "PUT", "/api/tasks/1", big, "Bearer secret"); rec.Code != 413 {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

// Calls the handlers directly (not through Routes()) so this test only exercises the
// handler/store locking. Run it with: go test -race ./internal/api -run Concurrent
func TestConcurrentPutsAndGets(t *testing.T) {
	defer guard(t)
	srv, _ := newTestServer("secret")
	call := func(handler http.HandlerFunc, method, path, body string) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		handler(httptest.NewRecorder(), req)
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				call(srv.handleTask, "PUT", "/api/tasks/1", `{"title":"t","status":"doing","priority":2,"tags":["a"]}`)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				call(srv.handleTask, "GET", "/api/tasks/1", "")
				call(srv.handleTasks, "GET", "/api/tasks?limit=5", "")
			}
		}()
	}
	wg.Wait()
}