package api

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"bughunt/internal/store"
)

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit == 0 {
			limit = 20
		}
		if page == 0 {
			page = 1
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
		json.NewDecoder(r.Body).Decode(&t)
		created := s.store.Create(&t)
		writeJSON(w, http.StatusOK, created)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/tasks/"))
	t := s.store.Get(id)

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, t)

	case http.MethodPut:
		var in store.Task
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		wasDone := false
		updated, ok := s.store.Update(id, func(t *store.Task){
			wasDone = t.Status == "done"
			t.Title = in.Title
			t.Description = in.Description
			t.Status = in.Status
			t.Priority = in.Priority
			t.Tags = in.Tags
			t.DueDate = in.DueDate
		})

		if !ok {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		if !wasDone && updated.Status == "done" {
			ev := Event{TaskID: updated.ID, Priority: updated.Priority}
			go func() { s.events <- ev } ()
		}
		writeJSON(w, http.StatusOK, t)

	case http.MethodDelete:
		s.store.Delete(id)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="tasks.csv"`)
	w.Write([]byte(s.store.ExportCSV()))
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
