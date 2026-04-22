# Owl Dictionary

[中文说明](README_ZH.md)

Owl is a web dictionary application for **MDX / MDD** files with a Go backend and a modern React frontend that is embedded into the Go server for single-binary deployment.

## Stack

- Backend: Go + Echo v5 + ent + SQLite (`github.com/lib-x/entsqlite`)
- Dictionary engine: `github.com/lib-x/mdx v0.1.9`
- Frontend: React + Vite + TypeScript + pnpm (embedded into Go via `go:embed`)
- Deployment: single Go service / single Docker image
- CI/CD: GitHub Actions for CI, release binaries, and Docker images

## Core product behavior

### Search model

- Guests can search **enabled public dictionaries**
- Signed-in users can search:
  - all enabled public dictionaries
  - plus their own private dictionaries
- Search results render MDX HTML and serve paired MDD resources (images/audio/CSS/fonts)
- Search UI includes:
  - best match highlighting
  - same-headword cross-dictionary comparison
  - public/private result grouping
  - search suggestions with keyboard navigation

### Dictionary maintenance

- Upload MDX and optional MDD files from the web UI
- Same basename `.mdx + .mdd` files are treated as one dictionary pair
- If a user uploads MDX first and adds MDD later, refresh can rediscover the pair
- Recursive library scanning supports Docker-mounted directories containing many dictionaries
- Dictionary status is tracked and shown in the UI:
  - `ok`
  - `missing_mdx`
  - `missing_mdd`
  - `missing_all`
- Maintenance actions:
  - refresh one dictionary
  - refresh the whole library
  - enable / disable
  - public / private toggle
  - delete
- Refresh returns a structured maintenance report: discovered / updated / skipped / failed

### User settings

Preferences are persisted on the backend per user:
- language (`zh-CN`, `en`)
- theme (`system`, `light`, `dark`, `sepia`)
- reading font mode (`sans`, `serif`, `mono`, `custom`)
- custom uploaded font

## Local development

### Backend

```bash
cd backend
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go test ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go vet ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go run ./cmd/server
```

Backend default address:
- `http://localhost:8080`

### Frontend

```bash
cd frontend
pnpm install
pnpm lint
pnpm build
pnpm dev
```

Frontend dev address:
- `http://localhost:3000`

For production-style local runs, `pnpm build` writes assets into `backend/web/dist`, and the Go server serves them via `go:embed`.
The dev server still proxies `/api` to the backend.

## Docker deployment

```bash
cp .env.example .env
docker compose up --build
```

Default address:
- Owl app + API: `http://localhost:8080`

The frontend is served by the Go backend from embedded assets.
Persistent data is stored in the Docker volume `owl_data`.

## Environment variables

See `.env.example`.

Important values:
- `OWL_JWT_SECRET`
- `OWL_BOOTSTRAP_ADMIN`
- `OWL_ADMIN_USERNAME`
- `OWL_ADMIN_PASSWORD`
- `OWL_DATA_DIR`
- `OWL_UPLOADS_DIR`
- `OWL_LIBRARY_DIR`
- `OWL_DB_PATH`

## Default admin bootstrap

If `OWL_BOOTSTRAP_ADMIN=true`, Owl creates the admin account on startup if missing.

Example defaults:
- username: `admin`
- password: `admin123456`

Change these before production use.

## API overview

### Public routes
- `GET /api/health`
- `GET /api/public/dictionaries`
- `GET /api/public/search?q=word&dict=id`
- `GET /api/public/suggest?q=prefix&dict=id`
- `GET /api/public/dictionaries/:id/resource/*`
- `POST /api/auth/register`
- `POST /api/auth/login`

### Authenticated routes
- `GET /api/me`
- `GET /api/preferences`
- `PUT /api/preferences`
- `POST /api/preferences/font`
- `GET /api/preferences/font`
- `GET /api/dictionaries`
- `POST /api/dictionaries/upload`
- `PATCH /api/dictionaries/:id`
- `PATCH /api/dictionaries/:id/public`
- `POST /api/dictionaries/:id/refresh`
- `POST /api/dictionaries/refresh`
- `DELETE /api/dictionaries/:id`
- `GET /api/dictionaries/:id/resource/*`
- `GET /api/search?q=word&dict=id`
- `GET /api/suggest?q=prefix&dict=id`

## CI / release automation

GitHub Actions included in `.github/workflows/`:

- `ci.yml`
  - frontend install / lint / build to embedded asset directory
  - backend test / vet
  - single-image Docker build verification
- `binary.yml`
  - tagged release binary builds with embedded frontend assets
  - draft GitHub Release asset upload
- `docker.yml`
  - tagged multi-arch single-image Docker builds and pushes

The standalone frontend container is no longer used in production.

## Verification status in this workspace

Verified locally:
- `go test ./...`
- `go vet ./...`
- `pnpm lint`
- `pnpm build`

Not verified in this environment:
- `docker compose up` runtime validation (Docker socket/buildx restrictions in this session)
- full end-to-end tests with real sample MDX/MDD dictionaries
