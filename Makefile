# Works in Git Bash / WSL / Linux / macOS (needs make). PowerShell users: see run.ps1
.PHONY: run build-c build-ts test race bench clean

build-c:
	$(MAKE) -C c

build-ts:
	cd frontend && npx tsc

run: build-c
	cd backend && go run ./cmd/server

race:
	cd backend && go run -race ./cmd/server

test:
	cd backend && go test ./...

bench:
	cd backend && go test -bench . -benchmem ./internal/store

clean:
	rm -rf c/bin backend/data
