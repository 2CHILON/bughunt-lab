package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func importJSON(srv *Server, body string) (int, string) {
	req := httptest.NewRequest("POST", "/api/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.handleImport(rec, req)
	return rec.Code, rec.Body.String()
}

func TestImportKeepsEachTasksOwnData(t *testing.T) {
	defer guard(t)
	srv, _ := newTestServer("secret")
	before := srv.store.Len()
	var items []string
	for i := 0; i < 20; i++ {
		items = append(items, fmt.Sprintf(`{"title":"imported %d","priority":%d}`, i, i%5+1))
	}
	code, body := importJSON(srv, "["+strings.Join(items, ",")+"]")
	if code != 201 {
		t.Fatalf("status = %d, want 201 (%s)", code, body)
	}
	var resp struct {
		Imported int
		IDs      []int
	}
	json.Unmarshal([]byte(body), &resp)
	if resp.Imported != 20 || len(resp.IDs) != 20 {
		t.Fatalf("response says imported=%d ids=%d, want 20/20 (%s)", resp.Imported, len(resp.IDs), body)
	}
	if got := srv.store.Len(); got != before+20 {
		t.Fatalf("store has %d tasks, want %d", got, before+20)
	}
	for i, id := range resp.IDs {
		task := srv.store.Get(id)
		if task == nil || task.Title != fmt.Sprintf("imported %d", i) {
			t.Fatalf("id %d: got %+v, want title %q", id, task, fmt.Sprintf("imported %d", i))
		}
	}
}

func TestImportIsAllOrNothing(t *testing.T) {
	defer guard(t)
	srv, _ := newTestServer("secret")
	before := srv.store.Len()
	code, body := importJSON(srv, `[{"title":"ok 0"},{"title":"ok 1"},{"title":"  "},{"title":"ok 3"}]`)
	if code != 400 || !strings.Contains(body, "task 2") {
		t.Fatalf("got %d %s, want 400 naming task 2", code, body)
	}
	if got := srv.store.Len(); got != before {
		t.Fatalf("a rejected import still added %d tasks", got-before)
	}
}

func TestImportRejectsEmptyTooManyAndNonArrays(t *testing.T) {
	defer guard(t)
	srv, _ := newTestServer("secret")
	many := "[" + strings.TrimSuffix(strings.Repeat(`{"title":"t"},`, 1001), ",") + "]"
	for name, body := range map[string]string{
		"empty list":    `[]`,
		"too many":      many,
		"not an array":  `{"title":"x"}`,
		"invalid json":  `[{nope`,
		"unknown field": `[{"titel":"x"}]`,
	} {
		if code, resp := importJSON(srv, body); code != 400 {
			t.Errorf("%s: status = %d, want 400 (%.80s)", name, code, resp)
		}
	}
}

// go test -race ./internal/api -run Concurrent
func TestConcurrentImportsAndReads(t *testing.T) {
	defer guard(t)
	srv, _ := newTestServer("secret")
	before := srv.store.Len()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				importJSON(srv, `[{"title":"a"},{"title":"b"}]`)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				srv.handleTasks(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/tasks?limit=10", nil))
			}
		}()
	}
	wg.Wait()
	if got, want := srv.store.Len(), before+4*20*2; got != want {
		t.Fatalf("store has %d tasks, want %d: imports were lost", got, want)
	}
}