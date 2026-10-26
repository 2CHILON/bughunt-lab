// Package api exposes the task store over HTTP.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
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

	quit         chan struct{} // closed by Close to stop the notifier
	notifierDone chan struct{} // closed by the notifier when it has exited
	closeOnce    sync.Once
}

func New(st *store.Store, token, staticDir, wordstatBin string) *Server {
	s := &Server{
		store:        st,
		token:        token,
		staticDir:    staticDir,
		wordstatBin:  wordstatBin,
		cache:        map[string]cacheEntry{},
		events:       make(chan Event, 64),
		quit:         make(chan struct{}),
		notifierDone: make(chan struct{}),
	}
	go s.runNotifier()
	return s
}

// notify queues an event without ever blocking the request. The notifier is best-effort:
// if its queue is full the event is dropped (and logged) instead of piling up goroutines.
func (s *Server) notify(ev Event) {
	select {
	case s.events <- ev:
	default:
		log.Printf("notifier: queue full, dropping event for task %d", ev.TaskID)
	}
}

// runNotifier delivers queued events until Close is called.
func (s *Server) runNotifier() {
	defer close(s.notifierDone)
	for {
		select {
		case ev := <-s.events:
			s.deliver(ev)
		case <-s.quit:
			return
		}
	}
}

// deliver pretends to send a webhook for one completed task. A return here only ends
// the handling of THIS event; it can no longer stop the whole notifier loop.
func (s *Server) deliver(ev Event) {
	if ev.Priority > 3 {
		log.Printf("notifier: webhook skipped for task %d (priority %d)", ev.TaskID, ev.Priority)
		return
	}
	log.Printf("notifier: webhook sent for task %d", ev.TaskID)
}

// Close stops the notifier goroutine. It is safe to call more than once.
func (s *Server) Close() {
	s.closeOnce.Do(func() { close(s.quit) })
	<-s.notifierDone
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

// writeJSON sends v as JSON with the given status. It marshals first, so an encoding
// failure becomes a clean 500 instead of a 200 with a half-written body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		log.Printf("writeJSON: cannot encode %T: %v", v, err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status) // headers must be set BEFORE this call
	w.Write(append(body, '\n'))
}

// writeError sends {"error": msg} as JSON with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	body, _ := json.Marshal(map[string]string{"error": msg})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	w.Write(append(body, '\n'))
}
