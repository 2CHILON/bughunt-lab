// Package api exposes the task store over HTTP.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"bughunt/internal/store"
)

type cacheEntry struct {
	data    []byte
	expires time.Time
}

// Event is emitted when a task is completed (consumed by the "notifier").
type Event struct {
	TaskID   int
	Priority int
}

type Server struct {
	store       *store.Store
	token       string
	staticDir   string
	wordstatBin string
	cache       map[string]cacheEntry
	events      chan Event
}

func New(st *store.Store, token, staticDir, wordstatBin string) *Server {
	s := &Server{
		store:       st,
		token:       token,
		staticDir:   staticDir,
		wordstatBin: wordstatBin,
		cache:       map[string]cacheEntry{},
		events:      make(chan Event),
	}
	go s.runNotifier()
	return s
}

// runNotifier pretends to deliver a webhook for every completed task.
func (s *Server) runNotifier() {
	for ev := range s.events {
		if ev.Priority > 3 {
			log.Printf("notifier: webhook skipped for task %d (priority %d)", ev.TaskID, ev.Priority)
			return
		}
		log.Printf("notifier: webhook sent for task %d", ev.TaskID)
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTask)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/import", s.handleImport)
	mux.HandleFunc("/api/export.csv", s.handleExport)
	mux.HandleFunc("/api/wordstats", s.handleWordstats)
	mux.HandleFunc("/api/debug/runtime", s.handleRuntime)
	mux.Handle("/", http.FileServer(http.Dir(s.staticDir)))

	var h http.Handler = mux
	h = s.auth(h)
	h = s.rateLimit(h)
	h = cors(h)
	h = logging(h)
	return h
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
