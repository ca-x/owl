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
  - lightweight source labels per result
  - public/private result grouping
  - backend-aggregated search suggestions with keyboard navigation

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
docker compose pull
docker compose up -d
```

Default address:
- Owl app + API: `http://localhost:8080`

The frontend is served by the Go backend from embedded assets.
Persistent data is stored in the Docker volume `owl_data`.
The compose files in this repository now use the published image `czyt/owl:latest` directly.

## Deployment guide

### Option 1: minimal single-service deployment

Use this when you want the smallest setup and are fine with SQLite + in-memory fuzzy fallback.

```bash
cp .env.example .env
# edit OWL_JWT_SECRET / admin credentials first
docker compose pull
docker compose up -d
```

This starts:
- Owl on `http://localhost:8080`
- SQLite inside the persistent `owl_data` volume
- uploaded dictionaries stored under `/app/data/uploads`

### Option 2: Redis + RediSearch deployment

Use this when you want Redis-backed exact/prefix indexes and RediSearch fuzzy lookup.

```bash
docker compose -f docker-compose.yml -f docker-compose.redis-stack.yml pull
docker compose -f docker-compose.yml -f docker-compose.redis-stack.yml up -d
```

Recommended behavior in this mode:
- exact/prefix index: Redis
- fuzzy search: RediSearch
- fallback: automatic fallback to in-memory fuzzy search if the module is unavailable

### Mounting an existing dictionary directory

If you already have a host directory full of `.mdx` / `.mdd` files, mount it into `OWL_LIBRARY_DIR`.

Example override:

```yaml
services:
  owl:
    environment:
      OWL_LIBRARY_DIR: /app/library
    volumes:
      - owl_data:/app/data
      - ./dicts:/app/library
```

After startup:
1. sign in
2. open **Manage**
3. click **Refresh library**

Owl will scan the mounted directory recursively and pair `name.mdx` with `name.mdd` automatically.

### Pure web-upload deployment

If you do not want a mounted dictionary directory, keep:
- `OWL_LIBRARY_DIR=/app/data/uploads`

Then all dictionaries can be managed from the web UI only.

### First boot checklist

After the container starts:
1. open `http://localhost:8080`
2. sign in with the bootstrap admin account
3. upload a test dictionary or mount one and refresh the library
4. optionally set dictionaries public/private in **Manage**
5. confirm search works from the home page

### How to verify the active search backend

Guest scope:

```bash
curl http://localhost:8080/api/public/search-backends
```

Authenticated scope:

```bash
curl -H 'Authorization: Bearer <token>' http://localhost:8080/api/debug/search-backends
```

Important fields:
- `fuzzy_backend: redisearch` → RediSearch is active
- `fuzzy_backend: memory-fuzzy` → fallback mode is active
- `prefix_backend: redis-prefix` → Redis prefix index is active

### Upgrade notes

When upgrading Owl:

```bash
git pull
docker compose down
docker compose pull
docker compose up -d
```

If you changed compose overlays, use the same overlay set during restart.

SQLite data and uploaded dictionaries remain in Docker volumes unless you remove them manually.

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
- `OWL_REDIS_ADDR` (optional)
- `OWL_REDIS_PASSWORD`
- `OWL_REDIS_DB`
- `OWL_REDIS_KEY_PREFIX`
- `OWL_REDIS_PREFIX_MAX_LEN`

## Optional Redis index cache

Owl can optionally use Redis as the MDX exact/prefix index cache layer.

Current behavior:
- when Redis is configured, Owl first tries RediSearch-based fuzzy lookup (scheme B)
- exact/prefix suggestion indexing is also stored in Redis
- grouped autocomplete suggestions are aggregated by the backend
- if RediSearch is unavailable, Owl automatically falls back to the in-memory mdx fuzzy store

Enable it by setting:
- `OWL_REDIS_ADDR=redis:6379`
- optional `OWL_REDIS_PASSWORD` / `OWL_REDIS_DB`
- optional `OWL_REDIS_KEY_PREFIX` / `OWL_REDIS_PREFIX_MAX_LEN`
- `OWL_REDIS_SEARCH_ENABLED=true`
- optional `OWL_REDIS_SEARCH_KEY_PREFIX`

Docker Compose example with Redis + RediSearch:

```bash
docker compose -f docker-compose.yml -f docker-compose.redis.yml pull
docker compose -f docker-compose.yml -f docker-compose.redis.yml up -d
```

Redis Stack example (recommended when you want the module bundle explicitly):

```bash
docker compose -f docker-compose.yml -f docker-compose.redis-stack.yml pull
docker compose -f docker-compose.yml -f docker-compose.redis-stack.yml up -d
```

Debug endpoints:
- guest scope: `GET /api/public/search-backends`
- authenticated scope: `GET /api/debug/search-backends`

These endpoints show whether each enabled dictionary is currently using:
- `redisearch`
- `memory-fuzzy`
- `redis-prefix`

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
- `GET /api/public/search-backends`
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
- `GET /api/debug/search-backends`

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
