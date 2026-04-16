# portflix

**A beautiful CLI tool to see what's running on your ports.**

Stop guessing which process is hogging port 3000. `portflix` gives you a color-coded table of every dev server, database, and background process listening on your machine -- with framework detection, Docker container identification, and interactive process management.

## What it looks like

```
$ ports

                  |    _| |_)
 __ \   _ \   __| __| |   | |\ \  /
 |   | (   | |    |   __| | | `  <
 .__/ \___/ _|   \__|_|  _|_| _/\_\
_|
  5 ports  dev only

┌───────┬─────────┬───────┬──────────────────────┬────────────┬────────┬───────────┐
│ PORT  │ PROCESS │ PID   │ PROJECT              │ FRAMEWORK  │ UPTIME │ STATUS    │
├───────┼─────────┼───────┼──────────────────────┼────────────┼────────┼───────────┤
│ :3000 │ node    │ 42872 │ frontend             │ Next.js    │ 1d 9h  │ ● healthy │
├───────┼─────────┼───────┼──────────────────────┼────────────┼────────┼───────────┤
│ :3001 │ node    │ 95380 │ preview-app          │ Next.js    │ 2h 40m │ ● healthy │
├───────┼─────────┼───────┼──────────────────────┼────────────┼────────┼───────────┤
│ :4566 │ docker  │ 58351 │ backend-localstack-1 │ LocalStack │ 10d 3h │ ● healthy │
├───────┼─────────┼───────┼──────────────────────┼────────────┼────────┼───────────┤
│ :5432 │ docker  │ 58351 │ backend-postgres-1   │ PostgreSQL │ 10d 3h │ ● healthy │
├───────┼─────────┼───────┼──────────────────────┼────────────┼────────┼───────────┤
│ :6379 │ docker  │ 58351 │ backend-redis-1      │ Redis      │ 10d 3h │ ● healthy │
└───────┴─────────┴───────┴──────────────────────┴────────────┴────────┴───────────┘

  5 ports active  ·  Run ports <number> for details  ·  --all to show everything
```

Colors: green = healthy, yellow = orphaned, red = zombie.

## Install

```bash
brew install mhrsntrk/portflix/portflix
```

## Usage

### Show dev server ports

```bash
ports
```

Shows dev servers, Docker containers, and databases. System apps (Spotify, Raycast, etc.) are filtered out by default.

### Show all listening ports

```bash
ports --all
```

Includes system services, desktop apps, and everything else listening on your machine.

### Inspect a specific port

```bash
ports 3000
# or
portflix 3000
```

Detailed view: process info, repository path, current git branch, memory usage, and an interactive prompt to kill the process.

### Kill a process

```bash
ports kill 3000                # kill by port
ports kill 3000 5173 8080      # kill multiple
ports kill 3000-3010           # kill a port range
ports kill 42872               # kill by PID
ports kill -f 3000             # force kill (SIGKILL)
```

Resolves port to process automatically. Falls back to PID if no listener matches. Use `-f` when a process won't die gracefully.

Port ranges expand into individual kills -- empty ports are silently skipped and shown as a summary:

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
ports logs 3000               # show last 50 lines and exit
ports logs 3000 -f            # follow (stream new lines)
```

Discovers log files automatically using `lsof` file descriptor detection. If stdout/stderr is redirected to a file, it finds and tails it. Falls back to system log (`log show` on macOS) when no log files are found.

```
$ ports logs 3000

  logs for :3000 (node, PID 42872)

  ▸ Tailing stdout: /tmp/next-dev.output

  ▲ Next.js 16.2.3 (Turbopack)
  - Local: http://localhost:3000
  ✓ Ready in 195ms
   GET / 200 in 990ms
   GET /api/auth/session 200 in 6ms
```

### Show all dev processes

```bash
ports ps
```

A developer-focused `ps`. Shows all running dev processes (not just port-bound ones) with CPU%, memory, framework detection, and a smart description column. Docker processes are collapsed into a single summary row.

```
$ ports ps

┌───────┬─────────┬──────┬──────────┬──────────┬───────────┬─────────┬────────────────────────────────┐
│ PID   │ PROCESS │ CPU% │ MEM      │ PROJECT  │ FRAMEWORK │ UPTIME  │ WHAT                           │
├───────┼─────────┼──────┼──────────┼──────────┼───────────┼─────────┼────────────────────────────────┤
│ 592   │ Docker  │ 1.3  │ 735.5 MB │ —        │ Docker    │ 13d 12h │ 14 processes                   │
├───────┼─────────┼──────┼──────────┼──────────┼───────────┼─────────┼────────────────────────────────┤
│ 36664 │ python3 │ 0.2  │ 17.6 MB  │ —        │ Python    │ 6d 10h  │ browser_use.skill_cli.daemon   │
├───────┼─────────┼──────┼──────────┼──────────┼───────────┼─────────┼────────────────────────────────┤
│ 26408 │ node    │ 0.1  │ 9.2 MB   │ —        │ Node.js   │ 10d 13h │ jest jest_runner_cloud.js      │
└───────┴─────────┴──────┴──────────┴──────────┴───────────┴─────────┴────────────────────────────────┘

  3 processes  ·  --all to show everything
```

```bash
ports ps --all    # show all processes, not just dev
```

### Clean up orphaned processes

```bash
ports clean
```

Finds and kills orphaned or zombie dev server processes. Only targets dev runtimes (node, python, etc.) -- won't touch your desktop apps.

### Watch for port changes

```bash
ports watch
```

Real-time monitoring that notifies you whenever a port starts or stops listening.

## How it works

Three shell calls, runs in ~0.2s:

1. **`lsof -iTCP -sTCP:LISTEN`** -- finds all processes listening on TCP ports
2. **`ps`** (single batched call) -- retrieves process details for all PIDs at once: command line, uptime, memory, parent PID, status
3. **`lsof -d cwd`** (single batched call) -- resolves the working directory of each process to detect the project and framework

For Docker ports, a single `docker ps` call maps host ports to container names and images.

Framework detection reads `package.json` dependencies and inspects process command lines. Recognizes Next.js, Vite, Express, Angular, Remix, Astro, Django, Rails, FastAPI, and many others. Docker images are identified as PostgreSQL, Redis, MongoDB, LocalStack, nginx, etc.

## Platform support

macOS only. No runtime required -- single native binary.

## License

[MIT](LICENSE)
