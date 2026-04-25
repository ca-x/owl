# Owl Dictionary

[中文说明](README_ZH.md)

Owl is a self-hosted web dictionary for **MDX / MDD** files. It gives you a browser-based dictionary workspace for personal lookup, shared public dictionaries, private user dictionaries, and MCP access for AI clients.

---

## What users can do

### Look up words

- Search MDX dictionaries directly in the browser.
- Guests can search enabled **public** dictionaries.
- Signed-in users can search public dictionaries plus their own private dictionaries.
- Search across all accessible dictionaries or filter to one dictionary.
- Use autocomplete suggestions with keyboard navigation.
- Open dictionary-internal links and jump to related entries.
- View MDX HTML content with paired MDD resources such as images, audio, CSS, fonts, and other media.
- Play audio resources from entries when available.
- Copy a clean plain-text version of a definition.
- Use recent searches; the retained count is configurable.

### Read comfortably

- Responsive search page for desktop and mobile.
- Mobile dictionary filter optimized for small screens.
- Search results scroll directly to the result area on mobile.
- The top result is highlighted, with additional matches grouped below.
- Public/private source labels make dictionary scope visible.
- Multiple themes are available, including retro-inspired, dark, and monochrome styles.
- Reading font can be switched between sans, serif, mono, or an uploaded custom font.

### Manage personal preferences

- Switch language between Simplified Chinese and English.
- Choose a visual theme.
- Configure reading font mode.
- Upload and select shared custom fonts.
- Set display name and avatar.
- Configure how many recent searches to keep.

---

## What dictionary managers can do

### Manage dictionaries

- Upload `.mdx` files and optional paired `.mdd` files from the web UI.
- Refresh a single dictionary after changing or adding files.
- Refresh the whole library to rediscover dictionaries from a mounted directory.
- Enable or disable dictionaries.
- Toggle dictionaries between public and private.
- Delete dictionaries.
- See file status in the UI:
  - `ok`
  - `missing_mdx`
  - `missing_mdd`
  - `missing_all`
- View structured refresh reports:
  - discovered
  - updated
  - skipped
  - failed

### Library scanning behavior

- Same-basename `.mdx + .mdd` files are treated as one dictionary pair.
- If an MDX is uploaded first and the matching MDD is added later, refresh can rediscover the pair.
- Mounted library directories are scanned recursively.
- Pure web-upload mode is also supported.

### Site-level settings

Administrators can manage:

- Whether new user registration is open.
- Optional footer extra information.
- Optional copyright text.

By default, footer text is not displayed unless configured.

---

## MCP support

Owl includes an SSE-based MCP server for AI clients.

### Endpoint

```text
/api/mcp/sse
```

Authentication is by a per-user MCP token:

```text
Authorization: Bearer <MCP_TOKEN>
```

For quick testing, the token can also be passed in the URL:

```text
/api/mcp/sse?token=<MCP_TOKEN>
```

The initial SSE connection must include the token. After the connection is established, SDK POST requests continue through the MCP session.

### MCP tools

- `list_dictionaries`
  - Lists dictionaries available to the token owner.
  - Scope: enabled public dictionaries plus that user's private dictionaries.

- `search_dictionary`
  - Searches dictionaries available to the token owner.
  - Accepts `query`.
  - Optional: `dictionary_id` or `dictionary_name`.
  - Optional: `format=markdown` returns Markdown text content for the MCP response; omit `format` to keep the default JSON text output.
  - If no dictionary is specified, Owl searches all dictionaries available to the token user, matching the web search scope.

### Token management

Each user can manage their own MCP token in the management UI:

- save a custom token
- generate a random token
- delete/revoke the token
- open a help dialog with usage examples

Tokens are stored hashed; after generation, only a short hint is shown later.

---

## Technical overview

### Stack

- Backend: Go + Echo v5 + ent
- Databases: SQLite by default, plus PostgreSQL and MySQL via configurable driver/DSN
- Dictionary engine: `github.com/lib-x/mdx`
- MCP server: `github.com/modelcontextprotocol/go-sdk`
- Frontend: React + Vite + TypeScript
- Search cache/index option: Redis + RediSearch
- Deployment: single Go service / single Docker image
- Frontend serving: production assets are embedded into the Go server with `go:embed`
- Automation: GitHub Actions for CI, release binaries, and Docker images

### Search backend behavior

Owl can run without Redis. In that mode it uses the local MDX search/index implementation.

When Redis is configured:

- exact/prefix indexes can be stored in Redis
- fuzzy lookup can use RediSearch
- autocomplete suggestions are aggregated by the backend
- if RediSearch is unavailable, Owl falls back automatically to the in-memory fuzzy store

---

## Docker deployment

This repository provides four Docker Compose files:

- `docker-compose.yml` — simplest deployment, SQLite, no Redis
- `docker-compose.redis.yml` — SQLite + Redis + RediSearch
- `docker-compose.postgres.yml` — PostgreSQL, no Redis
- `docker-compose.mysql.yml` — MySQL, no Redis

### Option 1: no Redis

```bash
cp .env.example .env
# edit OWL_JWT_SECRET and admin credentials first
docker compose -f docker-compose.yml pull
docker compose -f docker-compose.yml up -d
```

Default address:

```text
http://localhost:8080
```

This starts:

- Owl on `http://localhost:8080`
- SQLite in the persistent `owl_data` volume
- uploaded dictionaries under `/app/data/uploads`
- no Redis dependency

### Option 2: Redis + RediSearch

Use this when you want Redis-backed prefix/exact indexes and RediSearch fuzzy lookup.

```bash
cp .env.example .env
# edit OWL_JWT_SECRET and admin credentials first
docker compose -f docker-compose.redis.yml pull
docker compose -f docker-compose.redis.yml up -d
```

This starts:

- Owl on `http://localhost:8080`
- Redis Stack Server for Redis + RediSearch
- SQLite database and uploads in `owl_data`
- Redis data in `owl_redis`

### Option 3: PostgreSQL

Use this when you want Owl metadata in PostgreSQL instead of SQLite. Uploaded dictionary files still live in `owl_data`; only the relational database changes.

```bash
cp .env.example .env
# edit OWL_JWT_SECRET and admin credentials
docker compose -f docker-compose.postgres.yml pull
docker compose -f docker-compose.postgres.yml up -d
```

This starts PostgreSQL with a built-in `owl` database/user and sets:

```text
OWL_DB_TYPE=postgres
OWL_DB_DSN=postgres://...
```

### Option 4: MySQL

Use this when you want Owl metadata in MySQL instead of SQLite. Uploaded dictionary files still live in `owl_data`; only the relational database changes.

```bash
cp .env.example .env
# edit OWL_JWT_SECRET and admin credentials
docker compose -f docker-compose.mysql.yml pull
docker compose -f docker-compose.mysql.yml up -d
```

This starts MySQL with a built-in `owl` database/user and sets:

```text
OWL_DB_TYPE=mysql
OWL_DB_DSN=owl:...@tcp(mysql:3306)/owl?parseTime=true&charset=utf8mb4&loc=Local
```

The compose files use the published image `czyt/owl:latest`.

---

## Deployment guide

### Mount an existing dictionary directory

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

Owl scans the mounted directory recursively and pairs `name.mdx` with `name.mdd` automatically.

### Pure web-upload deployment

If you do not want a mounted dictionary directory, keep:

```text
OWL_LIBRARY_DIR=/app/data/uploads
```

Then dictionaries can be managed entirely from the web UI.

### First boot checklist

1. Open `http://localhost:8080`.
2. Sign in with the bootstrap admin account.
3. Upload a test dictionary, or mount a dictionary directory and refresh the library.
4. Set dictionaries public/private as needed.
5. Search from the home page.
6. Optionally configure registration, footer text, fonts, and MCP token access.

### Upgrade notes

Use the same compose file you started with.

No Redis:

```bash
git pull
docker compose -f docker-compose.yml down
docker compose -f docker-compose.yml pull
docker compose -f docker-compose.yml up -d
```

With Redis:

```bash
git pull
docker compose -f docker-compose.redis.yml down
docker compose -f docker-compose.redis.yml pull
docker compose -f docker-compose.redis.yml up -d
```

For PostgreSQL or MySQL, use the same pattern with `docker-compose.postgres.yml` or `docker-compose.mysql.yml`.

SQLite/PostgreSQL/MySQL data, uploaded dictionaries, and Redis data remain in Docker volumes unless you remove the volumes manually.

---

## Local development

### Backend

```bash
cd backend
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go test ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go vet ./...
GOPROXY=https://goproxy.cn,direct GOSUMDB=off go run ./cmd/server
```

Backend default address:

```text
http://localhost:8080
```

### Frontend

```bash
cd frontend
pnpm install
pnpm lint
pnpm build
pnpm dev
```

Frontend dev address:

```text
http://localhost:3000
```

For production-style local runs, `pnpm build` writes assets into `backend/web/dist`, and the Go server serves them through embedded assets. The Vite dev server proxies `/api` to the backend.

---

## Environment variables

See `.env.example` for the full list.

### Core service

- `OWL_PORT`
- `OWL_FRONTEND_ORIGIN`
- `OWL_JWT_SECRET`
- `OWL_DATA_DIR`
- `OWL_UPLOADS_DIR`
- `OWL_LIBRARY_DIR`

### Database

- `OWL_DB_TYPE` — `sqlite` by default; also supports `postgres` / `postgresql` and `mysql` / `mariadb`
- `OWL_DB_DSN` — database connection string; leave empty for SQLite to use the generated DSN from `OWL_DB_PATH`
- `OWL_DB_PATH` — SQLite database path used by the default SQLite DSN

When SQLite is used and `OWL_DB_DSN` is empty, Owl generates a DSN with shared cache, foreign keys, WAL journaling, normal synchronous mode, and a 10-second busy timeout. This improves concurrent read/write behavior while keeping the simple SQLite deployment path.

Examples:

```text
# SQLite default: leave DSN empty to derive from OWL_DB_PATH.
OWL_DB_TYPE=sqlite
OWL_DB_DSN=

# SQLite explicit DSN equivalent.
OWL_DB_TYPE=sqlite
OWL_DB_DSN=file:/app/data/data.db?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)

OWL_DB_TYPE=postgres
OWL_DB_DSN=postgres://owl:secret@postgres:5432/owl?sslmode=disable

OWL_DB_TYPE=mysql
OWL_DB_DSN=owl:secret@tcp(mysql:3306)/owl?parseTime=true&charset=utf8mb4&loc=Local
```

### Bootstrap and registration

- `OWL_BOOTSTRAP_ADMIN`
- `OWL_ADMIN_USERNAME`
- `OWL_ADMIN_PASSWORD`
- `OWL_ALLOW_REGISTER`

### Redis / RediSearch

- `OWL_REDIS_ADDR`
- `OWL_REDIS_PASSWORD`
- `OWL_REDIS_DB`
- `OWL_REDIS_KEY_PREFIX`
- `OWL_REDIS_PREFIX_MAX_LEN`
- `OWL_REDIS_SEARCH_ENABLED`
- `OWL_REDIS_SEARCH_KEY_PREFIX`

### Audio resources

- `OWL_AUDIO_CACHE_DIR`
- `FFMPEG_BIN`

---

## Debug endpoints

### Search backend status

Guest scope:

```bash
curl http://localhost:8080/api/public/search-backends
```

Authenticated scope:

```bash
curl -H 'Authorization: Bearer <token>' http://localhost:8080/api/debug/search-backends
```

Important fields:

- `fuzzy_backend: redisearch` — RediSearch is active
- `fuzzy_backend: memory-fuzzy` — fallback mode is active
- `prefix_backend: redis-prefix` — Redis prefix index is active
