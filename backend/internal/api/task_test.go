package api

import "testing"

func TestTaskByID(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	cases := []struct {
		name, method, path, body string
		want                     int
	}{
		{"get existing", "GET", "/api/tasks/1", "", 200},
		{"get missing", "GET", "/api/tasks/99999", "", 404},
		{"get not a number", "GET", "/api/tasks/abc", "", 400},
		{"get empty id", "GET", "/api/tasks/", "", 400},
		{"get id zero", "GET", "/api/tasks/0", "", 400},
		{"get negative id", "GET", "/api/tasks/-1", "", 400},
		{"get leading zero", "GET", "/api/tasks/01", "", 400},
		{"get plus sign", "GET", "/api/tasks/+1", "", 400},
		{"get extra path", "GET", "/api/tasks/1/extra", "", 400},
		{"get huge number", "GET", "/api/tasks/99999999999999999999", "", 400},
		{"put missing", "PUT", "/api/tasks/99999", `{"title":"x"}`, 404},
		{"put bad id", "PUT", "/api/tasks/abc", `{"title":"x"}`, 400},
		{"delete missing", "DELETE", "/api/tasks/99999", "", 404},
		{"delete bad id", "DELETE", "/api/tasks/abc", "", 400},
		{"delete existing", "DELETE", "/api/tasks/2", "", 204},
		{"post on an item", "POST", "/api/tasks/1", `{}`, 405},
	}
	for _, c := range cases {
		auth := ""
		if c.method != "GET" {
			auth = "Bearer secret"
		}
		rec := do(h, c.method, c.path, c.body, auth)
		if rec.Code != c.want {
			t.Errorf("%s: %s %s -> %d, want %d (body %q)", c.name, c.method, c.path, rec.Code, c.want, rec.Body.String())
		}
	}
}

func TestDeleteThenGetAndDeleteAgain(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	if rec := do(h, "DELETE", "/api/tasks/3", "", "Bearer secret"); rec.Code != 204 {
		t.Fatalf("first delete = %d, want 204", rec.Code)
	}
	if rec := do(h, "GET", "/api/tasks/3", "", ""); rec.Code != 404 {
		t.Fatalf("get after delete = %d, want 404", rec.Code)
	}
	if rec := do(h, "DELETE", "/api/tasks/3", "", "Bearer secret"); rec.Code != 404 {
		t.Fatalf("second delete = %d, want 404", rec.Code)
	}
}

func TestNotFoundAndMethodErrorsAreJSON(t *testing.T) {
	defer guard(t)
	_, h := newTestServer("secret")
	for _, req := range [][2]string{{"GET", "/api/tasks/99999"}, {"POST", "/api/tasks/1"}, {"DELETE", "/api/tasks"}} {
		rec := do(h, req[0], req[1], "", "Bearer secret")
		if ct := rec.Result().Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s %s: Content-Type = %q (status %d)", req[0], req[1], ct, rec.Code)
		}
	}
	rec := do(h, "PATCH", "/api/tasks/1", "", "Bearer secret")
	if rec.Code != 405 || rec.Result().Header.Get("Allow") == "" {
		t.Errorf("405 must carry an Allow header, got %d %q", rec.Code, rec.Result().Header.Get("Allow"))
	}
}