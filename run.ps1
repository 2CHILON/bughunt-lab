# PowerShell helper:  .\run.ps1 [run|race|test|bench|build-c|build-ts]
param([string]$Task = "run")
$root = $PSScriptRoot
switch ($Task) {
  "build-c"  { Push-Location "$root\c"; .\build.ps1; Pop-Location }
  "build-ts" { Push-Location "$root\frontend"; npx tsc; Pop-Location }
  "run"      { Push-Location "$root\backend"; go run ./cmd/server; Pop-Location }
  "race"     { Push-Location "$root\backend"; go run -race ./cmd/server; Pop-Location }   # -race needs gcc (cgo)
  "test"     { Push-Location "$root\backend"; go test ./...; Pop-Location }
  "bench"    { Push-Location "$root\backend"; go test -bench . -benchmem ./internal/store; Pop-Location }
}
