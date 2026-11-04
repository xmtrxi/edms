# EDMS — Electronic Document Management System

A full-stack document management system with a **Go** backend and **Nuxt 4 / Vue 3** frontend.

## Tech Stack

| Layer       | Backend                    | Frontend                         |
|-------------|----------------------------|----------------------------------|
| Language    | Go 1.26                    | TypeScript                       |
| Framework   | Echo (Labstack)            | Nuxt 4 (Vue 3)                   |
| Database    | SurrealDB                  | —                                |
| Storage     | S3 / Azure Blob            | —                                |
| Search      | Internal search module     | —                                |
| UI          | —                          | Tailwind CSS v4, shadcn-vue      |
| Formatting  | `gofmt`                    | Oxfmt                            |
| Linting     | `golangci-lint`            | Oxlint                           |

## Prerequisites

- **Go** >= 1.27 (see `backend/go.mod`)
- **Node.js** >= 24
- **SurrealDB** 3.x
- **pnpm** (install via `npm install -g pnpm`)

## Project Structure

```
edms/
├── backend/          # Go backend (Dockerfile, build context ./backend)
│   ├── cmd/api/      # API server and background worker
│   └── internal/     # Handlers, services, migrations, integrations
├── frontend/         # Nuxt 4 frontend (Dockerfile, build context ./frontend)
│   └── app/          # Pages, components, services
├── loadtest/         # k6 load tests
├── scripts/          # backup.sh and restore.sh
└── README.md
```

## Setup

### Backend

```bash
cd backend
go mod download
go run ./cmd/api           # Start the server
```

### Frontend

```bash
cd frontend
pnpm install
pnpm dev                   # Dev server at http://localhost:3000
```

### Docker

Each app has its own image; build them from their folders:

```bash
docker build -t edms-api backend
docker build -t edms-web frontend
```

- **API** (`edms-api`): runs as an unprivileged user, listens on `8080`, keeps
  local files and upload staging under the `/data` volume, and has a built-in
  health check (`/api healthcheck`). Configure it with the variables in
  `backend/.env.example`.
- **Web** (`edms-web`): Node server on `3000`. `NUXT_PUBLIC_API_BASE` is read
  at start-up, so one image works for every environment.

## Tests

```bash
cd backend && go test ./...
```

Integration tests run against real services when these are set, and are
skipped otherwise:

```bash
surreal start --user root --pass root memory --bind 127.0.0.1:8099 &
valkey-server --port 6399 &
EDMS_TEST_SURREALDB_URL=ws://127.0.0.1:8099/rpc \
EDMS_TEST_VALKEY_URL=redis://127.0.0.1:6399 go test ./...
```

CI (`.github/workflows/ci.yml`) runs formatting, vet, all tests with SurrealDB
and Valkey, golangci-lint, the frontend lint and build, and both Docker builds.

## Operations

- **Health:** `GET /health` reports each dependency and which features are on.
- **Metrics:** Prometheus at `GET /metrics` (protected by
  `MONITORING_USER`/`MONITORING_PASSWORD` when set). Includes request
  latency by route, uploads, malware scans, emails, PDF jobs, sign-ins and
  permission-check latency.
- **Tracing:** set `OTEL_EXPORTER_OTLP_ENDPOINT` to export OpenTelemetry
  traces for requests and background tasks.
- **Task queue:** dashboard at `/monitoring/tasks` (needs the monitoring
  credentials). Administrators of self-hosted installs also get
  **Administration → Integrations**, which shows stuck work and dead-letter
  tasks and retries them. Stuck work is also retried every hour.
- **Backups:** `scripts/backup.sh` exports SurrealDB and archives stored files
  (local or S3) with checksums; `scripts/restore.sh` restores into an empty
  install and refuses a backup whose checksums don't match. Run the backup on
  a schedule (for example nightly from cron or a Kubernetes CronJob), copy the
  backup folder off the host, and test a restore regularly. Valkey and
  Meilisearch need no backup: rebuild search with `POST /api/search/reindex`.
- **Load tests:** see [loadtest/README.md](loadtest/README.md).

## Code Formatting

### Backend (Go)

Format all Go code with `gofmt` (included with Go):

```bash
# Format in-place
gofmt -l -w backend/

# Check formatting only (CI-friendly, exits non-zero if unformatted)
gofmt -l backend/
```

If `gofumpt` is available (stricter formatter), use it instead:

```bash
gofumpt -l -w backend/
```

### Frontend (Nuxt / Vue / TypeScript)

Formatting is handled by **Oxfmt**, configured via `.oxfmtrc.json`:

```bash
cd frontend
pnpm fmt          # Format all files in-place
pnpm fmt:check    # Check formatting (CI-friendly)
```

### Linting

```bash
# Backend (requires golangci-lint)
cd backend && golangci-lint run ./...

# Frontend
cd frontend && pnpm lint        # Check lint errors
cd frontend && pnpm lint:fix    # Auto-fix
```

## Useful Commands

```bash
# Format everything before committing
gofmt -l -w backend/ && cd frontend && pnpm fmt

# Full CI check locally
gofmt -l backend/ && cd frontend && pnpm fmt:check && pnpm lint
```

## Development Workflow

1. Make changes
2. Run formatters: `gofmt -l -w backend/` + `pnpm fmt` (in `frontend/`)
3. Run linters: `pnpm lint` (in `frontend/`)
4. Run tests: `go test ./...` (in `backend/`) and `pnpm build` (in `frontend/`)
5. Commit

## Integration status

See [INTEGRATION_STATUS.md](INTEGRATION_STATUS.md) for the implemented visitor workflow, current frontend/backend integration coverage, deployment requirements, and the remaining mock-backed or infrastructure-dependent work.
