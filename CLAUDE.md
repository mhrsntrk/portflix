# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o ports .

# Run directly
go run . [args]

# Run a specific command
go run . ps
go run . kill 3000
go run . logs 3000 -f
go run . watch
go run . 3000

# Tidy dependencies
go mod tidy
```

No test suite exists. Manual invocation is the primary way to verify changes.

## Architecture

Go 1.22, single module at `github.com/mhrsntrk/portflix`. No build step — `go build` produces a single binary.

**Packages:**

```
main.go                        — cobra CLI wiring + non-TUI commands (kill, clean, inspect)
internal/scanner/
  scanner.go                   — all macOS shell calls (lsof, ps, docker ps); Port/Process types
  framework.go                 — IsDevProcess, DetectFramework*, framework color map
internal/tui/
  styles.go                    — lipgloss styles, fwColored(), statusDot(), pad/trunc helpers
  ports.go                     — interactive port list (Bubble Tea); RunPorts() returns logsPort
  ps.go                        — interactive process list (Bubble Tea)
  watch.go                     — live port-change monitor (Bubble Tea)
  logs.go                      — log tail viewer (bubbles/viewport + goroutine for -f)
```

**Data flow:**

`scanner.GetListeningPorts()` makes three batched shell calls:
1. `lsof -iTCP -sTCP:LISTEN -P -n` → raw port/PID list
2. `ps -p <pids> -o pid=,ppid=,stat=,rss=,etime=,command=` → process info for all PIDs at once
3. `lsof -a -d cwd -p <pids>` → working directories for all PIDs at once

Docker ports get an extra `docker ps` call if any docker process is detected.

`etime` (elapsed time, format `[[DD-]HH:]MM:SS`) is used instead of lstart to avoid macOS date-string parsing complexity.

**TUI models:**

All interactive commands use `tea.WithAltScreen()`. The ports list model has states: `screenList` → `screenDetail` / `screenConfirm`. Pressing `l` sets `LogsPort` and quits; `main.go` then calls `RunLogs`. Auto-refresh fires every 5 seconds via `tea.Tick`.

**Process status logic:**
- `zombie` = `stat` field contains `Z`
- `orphaned` = `ppid == 1` AND `IsDevProcess(name, cmd)` is true
- otherwise `healthy`

## Homebrew

Formula lives in `https://github.com/mhrsntrk/homebrew-portflix`.

```bash
brew tap mhrsntrk/portflix
brew install portflix
```

Install produces two binaries: `ports` (primary) and `portflix` (alias).

**Release workflow:** create a git tag (`git tag v1.x.x && git push --tags`), download the archive, compute `shasum -a 256`, update `url` + `sha256` in `homebrew-portflix/Formula/portflix.rb`.
