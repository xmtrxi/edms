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

- **Go** >= 1.26
- **Node.js** >= 18
- **pnpm** (install via `npm install -g pnpm`)

## Project Structure

```
edms/
├── backend/          # Go backend
│   ├── cmd/          # Entry points
│   ├── internal/     # Application logic
│   ├── migrations/   # Database migrations
│   ├── pkg/          # Shared packages
│   └── tests/        # Test suites
├── frontend/         # Nuxt 4 frontend
│   ├── app/          # Vue SFC source
│   └── ...
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
4. Run tests: `go test ./...` (in `backend/`)
5. Commit

## Integration status

See [INTEGRATION_STATUS.md](INTEGRATION_STATUS.md) for the implemented visitor workflow, current frontend/backend integration coverage, deployment requirements, and the remaining mock-backed or infrastructure-dependent work.
