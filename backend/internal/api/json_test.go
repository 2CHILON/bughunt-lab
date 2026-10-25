package api

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSONSendsContentTypeAndStatus(t *testing.T) {
	defer guard(t)
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]int{"a": 1})
	res := rec.Result() // what was really sent; rec.Header() is the live map and hides this bug
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("body is not valid JSON: %q", rec.Body.String())
	}
}

func TestWriteJSONEncodeFailureIs500(t *testing.T) {
	defer guard(t)
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]float64{"avg": math.NaN()}) // json cannot encode NaN
	res := rec.Result()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
	if !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("body should be a JSON error, got %q", rec.Body.String())
	}
}

func TestWriteErrorIsJSON(t *testing.T) {
	defer guard(t)
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "bad input")
	res := rec.Result()
	if res.StatusCode != 400 || res.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("got %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
}