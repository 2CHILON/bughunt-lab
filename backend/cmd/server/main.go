package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof" // pprof on the debug port, see README
	"os"
	"os/signal"
	"syscall"

	"bughunt/internal/api"
	"bughunt/internal/store"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "HTTP listen address")
	data := flag.String("data", "data/tasks.json", "path of the JSON data file")
	static := flag.String("static", "../frontend", "directory with the frontend")
	wordstat := flag.String("wordstat", "../c/bin/wordstat", "path of the C wordstat binary")
	seed := flag.Int("seed", 300, "seed this many tasks when the store is empty")
	pprofAddr := flag.String("pprof", "localhost:6060", "pprof listen address ('' to disable)")
	flag.Parse()

	if err := os.MkdirAll("data", 0o755); err != nil {
		log.Fatal(err)
	}

	st := store.New(*data)
	if st.Len() == 0 && *seed > 0 {
		log.Printf("seeding %d tasks...", *seed)
		st.Seed(*seed)
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigc
		log.Println("shutting down: flushing store to disk...")
		if err := st.Close(); err != nil {
			log.Printf("flush on shutdown failed: %v", err)
		}
		os.Exit(0)
	}()

	if *pprofAddr != "" {
		go func() { log.Println(http.ListenAndServe(*pprofAddr, nil)) }()
	}

	srv := api.New(st, os.Getenv("BUGHUNT_TOKEN"), *static, *wordstat)
	log.Printf("listening on http://%s (pprof on http://%s/debug/pprof/)", *addr, *pprofAddr)
	log.Fatal(http.ListenAndServe(*addr, srv.Routes()))
}
