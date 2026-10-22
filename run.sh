#!/usr/bin/env bash
# Git Bash / Linux / macOS:  ./run.sh [run|race|test|bench]
set -e
cd "$(dirname "$0")"
case "${1:-run}" in
  run)   make -C c || echo "(C build skipped: /api/wordstats will return 503)"; cd backend && go run ./cmd/server ;;
  race)  cd backend && go run -race ./cmd/server ;;
  test)  cd backend && go test ./... ;;
  bench) cd backend && go test -bench . -benchmem ./internal/store ;;
esac
