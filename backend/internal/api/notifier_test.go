package api

import (
	"io"
	"log"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompletingTasksDoesNotLeakGoroutines(t *testing.T) {
	defer guard(t)
	log.SetOutput(io.Discard) // the notifier logs every event
	defer log.SetOutput(os.Stderr)

	srv, _ := newTestServer("secret")
	defer srv.Close()
	put := func(status string) {
		body := `{"title":"t","status":"` + status + `","priority":5}`
		req := httptest.NewRequest("PUT", "/api/tasks/1", strings.NewReader(body))
		srv.handleTask(httptest.NewRecorder(), req)
	}

	before := runtime.NumGoroutine()
	for i := 0; i < 100; i++ { // 100 completions of a priority-5 task
		put("done")
		put("todo")
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before+5 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before+5 {
		t.Fatalf("goroutines grew from %d to %d: events are piling up with nobody to receive them", before, got)
	}
}

func TestNotifierKeepsRunningAfterHighPriorityEvent(t *testing.T) {
	defer guard(t)
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)
	srv, _ := newTestServer("secret")
	defer srv.Close()

	srv.notify(Event{TaskID: 1, Priority: 5}) // used to kill the loop
	for i := 0; i < 10; i++ {
		srv.notify(Event{TaskID: 2, Priority: 1})
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(srv.events) > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := len(srv.events); n != 0 {
		t.Fatalf("%d events are still queued: nobody is consuming them", n)
	}
}

func TestNotifyNeverBlocksWhenQueueIsFull(t *testing.T) {
	defer guard(t)
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)
	srv, _ := newTestServer("secret")
	srv.Close() // no consumer any more: the queue can only fill up
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			srv.notify(Event{TaskID: i})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("notify blocked on a full queue")
	}
}

func TestCloseIsIdempotentAndStopsTheNotifier(t *testing.T) {
	defer guard(t)
	srv, _ := newTestServer("secret")
	finished := make(chan struct{})
	go func() { srv.Close(); srv.Close(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return")
	}
}
