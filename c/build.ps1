# Windows PowerShell build (needs gcc on PATH, e.g. MSYS2 / MinGW-w64 / WinLibs)
New-Item -ItemType Directory -Force -Path bin | Out-Null
gcc -g -O0 -Wall -Wextra -o bin/wordstat.exe wordstat.c
if ($LASTEXITCODE -eq 0) { Write-Host "built bin/wordstat.exe" }
