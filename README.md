# bughunt-lab

A small, runnable, **intentionally buggy** task-board project for practising debugging,
profiling, optimisation and code review. Everything runs on `localhost`.

| Part | Language | Where | Role |
|---|---|---|---|
| Backend (focus of the lab) | **Go** (stdlib only, no downloads) | `backend/` | REST API, JSON-file persistence, middleware, pprof |
| Word counter | **C** | `c/` | CLI the backend shells out to (`GET /api/wordstats`) |
| Frontend | **TypeScript** (compiled to `frontend/dist`, no framework) | `frontend/` | Task board UI + debug panel |
| Tools | **JavaScript** (Node 18+) | `tools/` | Load tester and bulk seeder |

There are about 40 planted problems: **24 in the Go backend**, 6 in C, 5 in TypeScript, 4 in the Node tools.
`BUGS_ANSWER_KEY.md` lists all of them with file and line numbers - resist opening it until you have your own list.

## Architecture

```
 Browser (TS, dist/main.js)  --fetch-->  Go server :8080
                                          |  logging -> cors -> rateLimit -> auth -> mux
                                          |        handlers (internal/api)
                                          |             |-- store (internal/store) <-> data/tasks.json
                                          |             |-- notifier goroutine (task done events)
                                          |             '-- exec ../c/bin/wordstat   (C)
 pprof :6060 (localhost only)            '-- also serves frontend/ as static files

 tools/loadtest.js, tools/seed.js  --HTTP-->  Go server
```

Endpoints: `GET/POST /api/tasks`, `GET/PUT/DELETE /api/tasks/{id}`, `GET /api/stats`,
`POST /api/import`, `GET /api/export.csv`, `GET /api/wordstats`, `GET /api/debug/runtime`.

## Prerequisites

- **Go 1.21+** (required)
- **gcc** (optional): builds the C program, and enables `go run -race` / `go test -race`
  (on Windows use MSYS2 / MinGW-w64 / WinLibs; without it `/api/wordstats` just answers 503)
- **Node 18+** (optional): only for `tools/*.js`, and to rebuild the TypeScript. The compiled `frontend/dist` is included.

## Run it

Git Bash / WSL / Linux / macOS:

```bash
./run.sh            # builds the C tool if gcc exists, starts the server
# or: make run
```

PowerShell:

```powershell
.\run.ps1 build-c   # optional, needs gcc on PATH
.\run.ps1 run
```

Open <http://localhost:8080>. On first start the server seeds 300 tasks into `backend/data/tasks.json`
(delete that file to reset; `-seed 5000` for a bigger data set, run `go run ./cmd/server -h` for all flags).

Optional: `BUGHUNT_TOKEN=dev-token` (PowerShell: `$env:BUGHUNT_TOKEN="dev-token"`) turns on token checking for
mutating requests. The frontend already sends `dev-token`. Read the auth middleware to see what happens when you don't set it.

TypeScript changes: `cd frontend && npm install && npm run watch`.

## The tests are your bug reports

```bash
cd backend
go vet ./...                          # already reports one real bug
go test ./...                         # 15 tests fail on purpose; each failure names a bug
go test -race ./internal/store -run Concurrent
```

Fix a bug, rerun, watch the test go green. A panic in a test is reported as a failure
(`PANIC: ...`) so one crash doesn't hide the others.

## Debugging toolbox

- **VS Code**: `.vscode/launch.json` has Delve (server + current test), Chrome (frontend), Node and gdb configs.
  Install the *Go* extension. In **IntelliJ/GoLand** create a *Go Build* config for `./cmd/server` with working dir `backend`.
- **Race detector**: `./run.sh race`, then use the UI while `node tools/seed.js 200` runs.
- **Panics**: the server has no `recover` middleware on purpose; read the stack trace in the terminal and
  notice the client gets an *empty reply*.
- **Goroutine leaks**: complete a few P4/P5 tasks in the UI, press *Go runtime stats*, then
  `curl "http://localhost:6060/debug/pprof/goroutine?debug=1"`.
- **CPU / heap profiles** (while generating load):
  ```bash
  node tools/loadtest.js "http://localhost:8080/api/tasks?q=deploy" 300 10 &
  go tool pprof -http=: "http://localhost:6060/debug/pprof/profile?seconds=15"
  go tool pprof -http=: http://localhost:6060/debug/pprof/heap
  ```
- **Benchmarks**: `go test -bench . -benchmem -cpuprofile cpu.out ./internal/store` then `go tool pprof -top cpu.out`.
- **C**: `make -C c asan` (AddressSanitizer), or valgrind on Linux/WSL. Example:
  `python3 -c "print('a'*100)" | ./c/bin/wordstat-asan 3`
- **Browser**: DevTools Network tab (throttle to *Slow 3G*), Console, and the Sources tab (source maps are emitted).

## Suggested exercises

1. **Warm-up** - make all `store` tests green. Add a test before each fix when none exists.
2. **Stability** - stop every panic path (nil, slice bounds, regex). Add a `recover` middleware that returns 500 and logs the stack.
3. **Concurrency** - get `go test -race` and `go run -race` clean (store lock discipline, cache, rate limiter, import).
4. **Leaks** - fix the notifier; prove it with the goroutine count staying flat.
5. **Performance** - measure first: `Search`, `Stats`, `ExportCSV`, `Create` with persistence (target: 10x or better on
   `BenchmarkSearch`; `Create` should not be O(n)). Record before/after numbers with `benchstat`.
6. **API hygiene** - status codes, validation, JSON content type, CORS, auth, graceful shutdown.
7. **Cross-language** - fix the C program, then make the Go caller robust (timeout, stderr, parsing).
8. **Review** - do the review pass with `REVIEW_CHECKLIST.md` *before* reading the answer key.
9. **Stretch** - replace the JSON file with SQLite, add pagination totals + tests, add Go 1.22 mux patterns,
   rewrite the frontend search with debounce + `AbortController`, add CI (`go vet`, `go test -race`).

## Git workflow suggestion

```bash
git init && git add . && git commit -m "Initial buggy baseline"
git checkout -b fix/b01-unique-ids          # one bug per branch / PR, review your own diff
```

## Layout

```
bughunt-lab/
  backend/    go.mod, cmd/server/main.go, internal/{store,api}
  c/          wordstat.c, Makefile, build.ps1
  frontend/   index.html, styles.css, src/*.ts, dist/*.js (built)
  tools/      loadtest.js, seed.js
  BUGS_ANSWER_KEY.md   REVIEW_CHECKLIST.md   run.sh   run.ps1   Makefile
```
