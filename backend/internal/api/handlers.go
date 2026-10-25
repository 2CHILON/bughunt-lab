package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"bughunt/internal/store"
)

const maxBodyBytes = 1 << 20 // 1 MiB

var validStatus = map[string]bool{"todo": true, "doing": true, "done": true}

// decodeJSON reads exactly one JSON object from the size-limited request body into dst.
// On failure it writes the error response itself and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		switch {
		case errors.As(err, &tooBig):
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		case errors.Is(err, io.EOF):
			writeError(w, http.StatusBadRequest, "request body is empty")
		default:
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		}
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF { // reject trailing data after the object
		writeError(w, http.StatusBadRequest, "request body must contain a single JSON object")
		return false
	}
	return true
}

// validateTask checks and normalises a task coming from a client. It returns a message
// for the first problem found, or "" if the task is acceptable.
func validateTask(t *store.Task) string {
	t.Title = strings.TrimSpace(t.Title)
	if t.Title == "" {
		return "title is required"
	}
	if utf8.RuneCountInString(t.Title) > 200 {
		return "title must be at most 200 characters"
	}
	if utf8.RuneCountInString(t.Description) > 2000 {
		return "description must be at most 2000 characters"
	}
	if t.Status == "" {
		t.Status = "todo"
	} else if !validStatus[t.Status] {
		return `status must be "todo", "doing" or "done"`
	}
	if t.Priority == 0 {
		t.Priority = 3
	} else if t.Priority < 1 || t.Priority > 5 {
		return "priority must be between 1 and 5"
	}
	if t.DueDate != "" {
		if _, err := time.Parse("2006-01-02", t.DueDate); err != nil {
			return "dueDate must look like 2026-12-31"
		}
	}
	tags := make([]string, 0, len(t.Tags))
	for _, tag := range t.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > 30 {
			return "each tag must be at most 30 characters"
		}
		tags = append(tags, tag)
	}
	if len(tags) > 10 {
		return "at most 10 tags are allowed"
	}
	t.Tags = tags
	return ""
}
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		page, err := intParam(r, "page", 1, 1, 100000)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		limit, err := intParam(r, "limit", 20, 1, 100)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if term := q.Get("q"); term != "" {
			writeJSON(w, http.StatusOK, map[string]any{"items": s.store.Search(term), "page": 1, "limit": limit})
			return
		}
		items := s.store.List(store.Filter{
			Status: q.Get("status"),
			Tag:    q.Get("tag"),
			Page:   page,
			Limit:  limit,
		})
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": page, "limit": limit})

	case http.MethodPost:
		var t store.Task
		if !decodeJSON(w, r, &t) {
			return
		}
		if msg := validateTask(&t); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		created := s.store.Create(&t)
		w.Header().Set("Location", fmt.Sprintf("/api/tasks/%d", created.ID))
		writeJSON(w, http.StatusCreated, created)

	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// taskID extracts the numeric ID from /api/tasks/{id}. It accepts only plain positive
// integers: no signs, no leading zeros (so 1 and 01 cannot name the same task), no extra path.
func taskID(path string) (int, bool) {
	raw := strings.TrimPrefix(path, "/api/tasks/")
	if raw == "" || len(raw) > 9 || raw[0] == '0' {
		return 0, false
	}
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	id, err := strconv.Atoi(raw)
	return id, err == nil
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	id, valid := taskID(r.URL.Path)
	if !valid {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		t := s.store.Get(id)
		if t == nil {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeJSON(w, http.StatusOK, t)

	case http.MethodPut:
		var in store.Task
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		wasDone := false
		updated, ok := s.store.Update(id, func(t *store.Task) {
			wasDone = t.Status == "done"
			t.Title = in.Title
			t.Description = in.Description
			t.Status = in.Status
			t.Priority = in.Priority
			t.Tags = in.Tags
			t.DueDate = in.DueDate
		})
		if !ok {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		if !wasDone && updated.Status == "done" {
			ev := Event{TaskID: updated.ID, Priority: updated.Priority}
			go func() { s.events <- ev }()
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if !s.store.Delete(id) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)

		default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var tasks []store.Task
	if err := json.NewDecoder(r.Body).Decode(&tasks); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var wg sync.WaitGroup
	imported := 0
	for _, t := range tasks {
		go func() {
			wg.Add(1)
			defer wg.Done()
			s.store.Create(&t)
			imported++
		}()
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]int{"imported": imported})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if e, ok := s.cache["stats"]; ok {
		w.Write(e.data)
		return
	}
	data, _ := json.Marshal(s.store.Stats())
	s.cache["stats"] = cacheEntry{data: data, expires: time.Now().Add(30 * time.Second)}
	w.Write(data)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="tasks.csv"`)
	if err := s.store.ExportCSV(w); err != nil {
		log.Printf("export: %v", err) // headers are already sent, so all we can do is log
	}
}

type wordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

// handleWordstats shells out to the C program in ../c and returns its top words.
func (s *Server) handleWordstats(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(s.wordstatBin); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "wordstat binary not built; run `make -C c` (see README)",
		})
		return
	}
	cmd := exec.Command(s.wordstatBin, "5")
	cmd.Stdin = strings.NewReader(s.store.Texts())
	out, _ := cmd.CombinedOutput()

	var res []wordCount
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, " ")
		n, _ := strconv.Atoi(parts[1])
		res = append(res, wordCount{Word: parts[0], Count: n})
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRuntime(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	writeJSON(w, http.StatusOK, map[string]any{
		"goroutines":  runtime.NumGoroutine(),
		"heapAllocMB": float64(m.HeapAlloc) / 1024 / 1024,
		"numGC":       m.NumGC,
		"tasks":       s.store.Len(),
	})
}

func intParam(r *http.Request, name string, def, min, max int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("invalid %s: must be an integer between %d and %d", name, min, max)
	}
	return n, nil
}