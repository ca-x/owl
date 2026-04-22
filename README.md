# Owl Dictionary

Owl is a web dictionary application for **MDX / MDD** files.

## Stack

- Backend: Go + Echo v5 + ent + SQLite (`github.com/lib-x/entsqlite`)
- Dictionary engine: `github.com/lib-x/mdx v0.1.5`
- Frontend: React + Vite + TypeScript
- Deployment: Docker + docker-compose

## Features

- User registration and login with JWT
- Personal dictionary library per user
- Admin bootstrap account support
- Upload MDX and optional paired MDD resource files
- Enable / disable / delete dictionaries
- Search across all enabled dictionaries with fuzzy matching
- Render MDX HTML definitions
- Serve MDD images/audio/CSS resources through the backend
- Responsive frontend with dark / light mode

## Local development

### Backend

```bash
cd backend
GOSUMDB=off GOPROXY=https://goproxy.cn,direct go test ./...
GOSUMDB=off GOPROXY=https://goproxy.cn,direct go run ./cmd/server
```

The backend listens on `http://localhost:8080`.

### Frontend

```bash
cd frontend
pnpm install
pnpm build
pnpm dev
```

The frontend dev server listens on `http://localhost:3000` and proxies `/api` to the backend.

## Docker deployment

```bash
cp .env.example .env
docker compose up --build
```

- Frontend: `http://localhost:3000`
- Backend API: `http://localhost:8080/api`

## Default admin bootstrap

If `OWL_BOOTSTRAP_ADMIN=true`, Owl creates the admin account on startup if it does not already exist.

Default example values:

- Username: `admin`
- Password: `admin123456`

Change these in `.env` before production use.

## API overview

- `POST /api/auth/register`
- `POST /api/auth/login`
- `GET /api/me`
- `GET /api/dictionaries`
- `POST /api/dictionaries/upload`
- `PATCH /api/dictionaries/:id`
- `DELETE /api/dictionaries/:id`
- `GET /api/dictionaries/:id/resource/*`
- `GET /api/search?q=word&dict=id`

## Notes

- SQLite runs in file mode with WAL-friendly pragmas.
- Uploaded dictionary files are stored in the persistent Docker volume.
- MDD assets referenced by MDX HTML are rewritten to backend resource URLs.
