# rad

**Review Apps Daemon** — a Go binary that manages the full lifecycle of Rails review apps on customer servers.

rad handles cloning repos, installing Ruby/Node, setting up databases, starting processes, configuring reverse proxies, and tearing everything down when you're done.

## Building

```bash
# Development build
make build

# The binary lands at bin/rad
./bin/rad --version
```

Requires Go 1.22+ and `CGO_ENABLED=0` (set automatically in the Makefile).

## Dev Mode

Run rad locally on macOS/Linux for development and testing:

```bash
# Start rad in dev mode (auto-generates stream token as "stream-secret")
./bin/rad --dev --token secret

# Or specify a custom stream token
./bin/rad --dev --token secret --stream-token my-stream-token

# In another terminal, deploy a test app
curl -X POST http://localhost:7890/apps/deploy \
  -H "Authorization: Bearer secret" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "my-test-app",
    "repo_url": "https://github.com/owner/repo.git",
    "branch": "main",
    "subdomain": "my-test-app",
    "callback_url": "http://localhost:3000/api/v1/builds/1/status"
  }'
```

Dev mode stores apps in `~/.reviewapps/apps/`, disables Caddy, and serves on `localhost:7890`.

## API Endpoints

All endpoints except `/health` require `Authorization: Bearer <token>`.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Server health, versions, uptime |
| `POST` | `/apps/deploy` | Deploy a review app (async, returns 202) |
| `GET` | `/apps` | List all apps |
| `GET` | `/apps/{id}/status` | App status, URL, memory, uptime |
| `GET` | `/apps/{id}/logs` | Build or runtime logs |
| `GET` | `/apps/{id}/logs/stream` | WebSocket log streaming (real-time) |
| `POST` | `/apps/{id}/restart` | Restart all processes |
| `POST` | `/apps/{id}/exec` | Run a command in app context |
| `DELETE` | `/apps/{id}` | Teardown and remove |
| `POST` | `/update` | Trigger self-update |

### WebSocket Log Streaming

The `/apps/{id}/logs/stream` endpoint provides real-time log streaming over WebSocket.

**Auth**: Uses a separate read-only `--stream-token` that's safe to embed in browser JavaScript (can only read log streams, not deploy/teardown/exec). The main `--token` also works. Pass the token via `?token=` query param since browser WebSocket APIs can't set custom headers.

**Query params**:
- `type=build` (default) — stream build logs during a deploy
- `type=runtime` — tail runtime logs (`tail -f` style)
- `process=web` (default) — which process to tail (runtime only)
- `token=<token>` — stream token or main token

```bash
# Stream build logs during deploy
websocat 'ws://localhost:7890/apps/my-app/logs/stream?type=build&token=secret'

# Tail runtime logs
websocat 'ws://localhost:7890/apps/my-app/logs/stream?type=runtime&token=stream-secret'

# Tail worker process logs
websocat 'ws://localhost:7890/apps/my-app/logs/stream?type=runtime&process=worker&token=stream-secret'
```

Build log streams send existing log lines as backlog, then stream new lines as they arrive. The connection closes automatically when the deploy finishes. Runtime log streams send the last 100 lines as backlog, then poll for new content.

## Deploy Pipeline

28 steps, executed serially:

1. Create app directory
2. Git clone (or fetch+reset on redeploy)
3. Parse `reviewapps.yml`
4. Branch filter check
5. Run `after_clone` hooks
6. Install system packages
7. Write Rails initializer
8. Install Ruby (via [rv](https://github.com/nicholasgasior/rv))
9. Bundle platform fix
10. Install gems
11. Install Node (via [fnm](https://github.com/Schniz/fnm))
12. Detect JS package manager
13. Install JS dependencies
14. Run `before_build` hooks
15. Setup databases
16. Write `.env` file
17. Run `before_migrate` hooks
18. `db:prepare` (or `db:migrate` on redeploy)
19. Asset precompile
20. Seed database
21. Run `after_build` hooks
22. Allocate port
23. Start processes
24. Health check
25. Configure Caddy reverse proxy
26. Run `after_deploy` hooks
27. Callback to web app

On failure, `on_failure` hooks run and a failure callback is sent.

## reviewapps.yml

Optional config file in the repo root:

```yaml
ruby: "3.4.1"
node: "22"
database: postgresql

# Multi-database support (Rails 7+/8)
databases:
  primary: postgresql
  queue: postgresql
  cache: sqlite
  cable: sqlite

app_path: "."  # monorepo support

build:
  command: "bin/rails assets:precompile"

setup:
  command: "bin/setup"  # alternative to db:prepare

seed:
  command: "bin/rails db:seed"

processes:
  web: bin/rails server -p $PORT
  worker: bundle exec sidekiq -c 2

hooks:
  after_clone:
    - "git-crypt unlock"
  before_build:
    - "echo 'starting build'"
  after_deploy:
    - "bin/rails runner 'AdminNotifier.deployed!'"
  on_failure:
    - "curl -X POST $SLACK_WEBHOOK -d '{\"text\":\"Deploy failed\"}'"

branches:
  only: "feature/*"    # glob pattern
  # ignore: "dependabot/*"

health_check:
  path: /up
  timeout: 30
  interval: 2

system_packages:
  - imagemagick
  - ffmpeg

env:
  RAILS_ENV: production
  RAILS_LOG_TO_STDOUT: "true"
```

## Self-Update

```bash
# Check for updates
rad update --check

# Update to latest
rad update

# Skip confirmation
rad update --force
```

The web app can also trigger updates via `POST /update`.

## Releasing

Trigger the release workflow from GitHub Actions:

1. Go to **Actions > Release > Run workflow**
2. Enter the version (e.g., `0.2.0`)
3. The workflow builds `linux/amd64` and `linux/arm64` binaries, creates a GitHub release with checksums

Or build locally:

```bash
make release VERSION=0.2.0
# Artifacts in dist/
```

## Architecture

- 23 internal packages, 57 Go files
- Single static binary, no runtime dependencies
- Serial build queue (one deploy at a time)
- Persistent state via `state.json`
- Process crash monitoring with auto-restart
- Caddy integration for reverse proxy + HTTPS

## License

Private — ReviewApps.dev
