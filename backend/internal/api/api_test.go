package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bughunt/internal/store"
)

func guard(t *testing.T) {
	t.Helper()
	if r := recover(); r != nil {
		t.Fatalf("PANIC: %v", r)
	}
}

func newTestServer(token string) (*Server, http.Handler) {
	st := store.New("")
	st.Seed(40)
	s := New(st, token, ".", "/does/not/exist")
	return s, s.Routes()
}

func do(h http.Handler, method, path, body, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestGetMissingTaskIs404(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	if rec := do(h, "GET", "/api/tasks/99999", "", ""); rec.Code != 404 {
		t.Fatalf("status = %d, want 404 (body %q)", rec.Code, rec.Body.String())
	}
}

func TestUpdateMissingTaskIs404(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	rec := do(h, "PUT", "/api/tasks/99999", `{"title":"x"}`, "Bearer secret")
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestCreateReturns201AndRejectsEmptyTitle(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	if rec := do(h, "POST", "/api/tasks", `{"title":"ok"}`, "Bearer secret"); rec.Code != 201 {
		t.Fatalf("valid create: status = %d, want 201", rec.Code)
	}
	if rec := do(h, "POST", "/api/tasks", `{"title":""}`, "Bearer secret"); rec.Code != 400 {
		t.Fatalf("empty title: status = %d, want 400", rec.Code)
	}
	if rec := do(h, "POST", "/api/tasks", `{not json`, "Bearer secret"); rec.Code != 400 {
		t.Fatalf("bad json: status = %d, want 400", rec.Code)
	}
}

func TestAuthRejectsMissingTokenWhenServerTokenUnset(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("") // BUGHUNT_TOKEN not configured
	if rec := do(h, "POST", "/api/tasks", `{"title":"sneaky"}`, ""); rec.Code != 401 {
		t.Fatalf("status = %d, want 401 when no token is configured", rec.Code)
	}
}

func TestJSONContentType(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	rec := do(h, "GET", "/api/tasks?limit=5&page=1", "", "")
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestNegativeLimitIsRejected(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	if rec := do(h, "GET", "/api/tasks?limit=-5", "", ""); rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestStatsReflectNewTasks(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	do(h, "GET", "/api/stats", "", "") // warm the cache
	do(h, "POST", "/api/tasks", `{"title":"new","priority":3}`, "Bearer secret")
	rec := do(h, "GET", "/api/stats", "", "")
	if !strings.Contains(rec.Body.String(), `"total":41`) {
		t.Fatalf("stats are stale: %s", rec.Body.String())
	}
}
