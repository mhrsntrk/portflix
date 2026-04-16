# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the CLI locally
node src/index.js
npm start

# Test specific commands
node src/index.js --all
node src/index.js ps
node src/index.js 3000
node src/index.js kill 3000
node src/index.js logs 3000
node src/index.js watch
node src/index.js clean
```

There are no tests or lint scripts. Manual invocation is the primary way to verify changes.

## Architecture

This is an ESM-only Node.js CLI (`"type": "module"`) with no build step. Entry point is `src/index.js`, which parses `process.argv` directly and dispatches to commands.

**Data flow for most commands:**
1. `src/platform/index.js` — lazy-loads `darwin.js` via dynamic `import()`
2. The platform module runs shell commands (`lsof`, `ps`, `docker ps`) and returns raw data
3. `src/scanner.js` — consumes raw platform data, enriches it with framework detection, uptime, project root resolution, and Docker mapping, then exports typed async functions
4. `src/display.js` — pure display layer using `chalk` + `cli-table3`; never calls scanner directly, receives data from `index.js`

**Key scanner concepts:**
- `getListeningPorts()` / `getAllProcesses()` batch all shell calls (single `ps -p pid1,pid2,...` and `lsof -a -d cwd -p ...`) to stay fast (~0.2s)
- Framework detection has three layers: command string (`detectFrameworkFromCommand`), `package.json` deps (`detectFramework`), Docker image name (`detectFrameworkFromImage`)
- `isDevProcess()` filters out system/desktop apps — both a blocklist (`systemApps`) and an allowlist (`devNames`)
- Orphaned process = `ppid === 1` and is a dev process; zombie = `stat` contains `Z`
- `resolveKillTarget(n)` tries port lookup first (if `n <= 65535`), falls back to PID

**Platform module** (`src/platform/darwin.js`) exports:
- `getListeningPortsRaw()` — returns `{ port, pid, processName }[]`
- `batchProcessInfo(pids)` — returns `Map<pid, { ppid, stat, rss, lstart, command }>`
- `batchCwd(pids)` — returns `Map<pid, string>`
- `getAllProcessesRaw()` — returns full process list for `ports ps`
- `getProcessTree(pid)` — walks parent chain upward

**CLI binary names:** `ports` and `portflix` (both map to `src/index.js`).

## Homebrew

The formula lives in the separate tap repo: https://github.com/mhrsntrk/homebrew-portflix

```
homebrew-portflix/
  Formula/
    portflix.rb   ← install source: npm tarball
```

Install:
```bash
brew tap mhrsntrk/portflix
brew install portflix
```

When releasing a new version: publish to npm, compute the new sha256 (`curl -sL <tarball-url> | shasum -a 256`), then update `url`, `sha256`, and version in `homebrew-portflix/Formula/portflix.rb`.
