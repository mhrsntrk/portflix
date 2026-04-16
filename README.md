# portflix

**A beautiful CLI tool to see what's running on your ports.**

Stop guessing which process is hogging port 3000. `portflix` gives you a color-coded table of every dev server, database, and background process listening on your machine -- with framework detection, Docker container identification, and interactive process management.

## Install

```bash
brew install mhrsntrk/portflix/portflix
```

## What it looks like

### Port list (`ports`)

```
                  |    _| |_)
 __ \   _ \   __| __| |   | |\ \  /
 |   | (   | |    |   __| | | `  <
 .__/ \___/ _|   \__|_|  _|_| _/\_\
_|

  PORT     │  PROCESS     │  PID    │  PROJECT               │  FRAMEWORK   │  UPTIME    │  STATUS
  ─────────────────────────────────────────────────────────────────────────────────────────────────
▶ :3000    │  node        │  42872  │  frontend              │  Next.js     │  1d 9h     │  ● healthy
  :3001    │  node        │  95380  │  preview-app           │  Next.js     │  2h 40m    │  ● healthy
  :4566    │  docker      │  58351  │  backend-localstack-1  │  LocalStack  │  10d 3h    │  ● healthy
  :5432    │  docker      │  58351  │  backend-postgres-1    │  PostgreSQL  │  10d 3h    │  ● healthy
  :6379    │  docker      │  58351  │  backend-redis-1       │  Redis       │  10d 3h    │  ● healthy

  5 ports  [dev mode]
  ↑↓/jk nav  enter detail  K kill  l logs  r refresh  a all  q quit
```

Status dots: `●` green = healthy, yellow = orphaned (ppid 1), red = zombie.

### Process list (`ports ps`)

```
                  |    _| |_)
 __ \   _ \   __| __| |   | |\ \  /
 |   | (   | |    |   __| | | `  <
 .__/ \___/ _|   \__|_|  _|_| _/\_\
_|

  PID      │  PROCESS     │  CPU%   │  MEM        │  PROJECT   │  FRAMEWORK   │  UPTIME    │  WHAT
  ──────────────────────────────────────────────────────────────────────────────────────────────────────────
▶ 592      │  Docker      │  1.3    │  735.5 MB   │  —         │  Docker      │  13d 12h   │  14 processes
  36664    │  python3     │  0.2    │  17.6 MB    │  —         │  Python      │  6d 10h    │  browser_use.daemon
  26408    │  node        │  0.1    │  9.2 MB     │  —         │  Node.js     │  10d 13h   │  jest runner

  3 processes  [dev mode]
  ↑↓/jk nav  a all  r refresh  q quit
```

## Usage

### Show dev server ports

```bash
ports
```

Shows dev servers, Docker containers, and databases. System apps (Spotify, Raycast, etc.) are filtered out. Press `a` to toggle between dev and all ports.

### Show all listening ports

```bash
ports --all
```

### Inspect a specific port

```bash
ports 3000
```

Shows process info, working directory, git branch, memory, uptime, and an interactive kill prompt.

### Kill a process

```bash
ports kill 3000                # kill by port
ports kill 3000 5173 8080      # kill multiple
ports kill 3000-3010           # kill a port range
ports kill 42872               # kill by PID
ports kill -f 3000             # force kill (SIGKILL)
```

Port ranges silently skip empty ports and print a summary:

```
$ ports kill 3000-3005

  Killing :3000 — node (PID 42872)
  ✓ Sent SIGTERM to :3000 — node (PID 42872)
  Killing :3001 — node (PID 95380)
  ✓ Sent SIGTERM to :3001 — node (PID 95380)

  Range summary: 2 killed, 4 empty
```

### View process logs

```bash
ports logs 3000       # show last 50 lines
ports logs 3000 -f    # follow (stream new lines)
```

Discovers log files automatically via `lsof` file descriptor detection. Falls back to `log show` (macOS system log) when no log files are found.

### Show all dev processes

```bash
ports ps         # dev processes only
ports ps --all   # everything
```

All running dev processes sorted by CPU, not just port-bound ones. Docker processes are collapsed into a single summary row.

### Clean up orphaned processes

```bash
ports clean
```

Finds and kills orphaned or zombie dev server processes (node, python, etc.).

### Watch for port changes

```bash
ports watch
```

Real-time monitor that prints a timestamped line whenever a port opens or closes.

## How it works

Three shell calls, runs in ~0.2s:

1. **`lsof -iTCP -sTCP:LISTEN`** — all TCP listeners
2. **`ps`** (batched) — process details for all PIDs at once: command, uptime, memory, parent PID, status
3. **`lsof -d cwd`** (batched) — working directory per process for project and framework detection

Docker ports are enriched via `docker ps`, mapping host ports to container names and images.

Framework detection reads `package.json` deps and inspects command lines. Recognizes Next.js, Vite, Express, Angular, Remix, Astro, Django, Rails, FastAPI, and many more.

## Platform support

macOS only. No runtime required — single native binary.

## License

[MIT](LICENSE)
