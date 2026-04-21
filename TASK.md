# Owl Dictionary - Development Task

## Project Overview
Build a web-based dictionary application called **Owl** that supports MDX/MDD dictionary files.
The app allows users to look up words and manage their own personal dictionaries.
Core dictionary engine: Go library `github.com/lib-x/mdx` (MDX/MDD parser with `io/fs.FS` support).

## Architecture
- **Backend**: Go (REST API)
- **Frontend**: Modern web UI (React with Vite, or Vue with Vite — choose one and be consistent)
- **Database**: SQLite (for user accounts, dictionary metadata)
- **Deployment**: Docker + docker-compose (single `docker-compose up` to run everything)

## Project Structure
```
owl/
├── backend/
│   ├── cmd/
│   │   └── server/main.go        # Entry point
│   ├── internal/
│   │   ├── api/                   # HTTP handlers, routes
│   │   ├── dictionary/            # MDX loading, querying logic
│   │   ├── user/                  # User auth (simple JWT or session)
│   │   └── models/                # Data models
│   ├── data/                      # SQLite DB, uploaded MDX files
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── components/            # UI components
│   │   ├── pages/                 # Home, Search, DictionaryManager
│   │   ├── services/              # API client
│   │   ├── App.tsx (or App.vue)
│   │   └── main.tsx (or main.ts)
│   ├── package.json
│   ├── vite.config.ts
│   ├── Dockerfile
│   └── nginx.conf                 # For production serving
├── docker-compose.yml
├── TASK.md                        # This file
└── README.md
```

## Core Features

### 1. Word Lookup (核心查询)
- Search bar on homepage, type a word and get definitions
- Support fuzzy/prefix matching (not just exact match)
- Show results from ALL loaded dictionaries, grouped by dictionary name
- HTML rendering of dictionary entries (MDX content is HTML)

### 2. Dictionary Management (词典管理)
- Upload MDX files (and optionally paired MDD resource files) via web UI
- List uploaded dictionaries with metadata (title, description, entry count)
- Enable/disable individual dictionaries
- Delete dictionaries
- Store uploaded files in a persistent volume

### 3. User System (用户系统)
- Simple registration/login (username + password, bcrypt hashed)
- JWT token auth for API
- Each user has their own set of dictionaries (isolated)
- Admin user can see all dictionaries

### 4. Docker Deployment
- `docker-compose.yml` with backend + frontend + volumes
- Backend listens on port 8080
- Frontend served by nginx on port 3000 (or proxied through backend)
- Persistent volume for: SQLite DB, uploaded MDX files
- `.env` file for configuration (JWT secret, ports, etc.)

## API Design (REST)
```
POST   /api/auth/register         # Register
POST   /api/auth/login            # Login, returns JWT
GET    /api/dictionaries          # List user's dictionaries
POST   /api/dictionaries/upload   # Upload MDX file
DELETE /api/dictionaries/:id      # Delete a dictionary
PATCH  /api/dictionaries/:id      # Toggle enable/disable
GET    /api/search?q=word         # Search across enabled dictionaries
GET    /api/search?q=word&dict=id # Search in specific dictionary
```

## lib-x/mdx Usage Reference
```go
import "github.com/lib-x/mdx"

// Load dictionary
mdict, err := mdx.New("path/to/file.mdx")
err = mdict.BuildIndex()

// Query
definition, err := mdict.Lookup("hello")

// Metadata
title := mdict.Title()
desc := mdict.Description()

// FS interface (for serving MDD resources)
fs := mdict.FS()
```

## Frontend Requirements
- Clean, modern design (use Tailwind CSS or similar)
- Dark/light mode toggle
- Responsive (mobile-friendly)
- Search results display with dictionary source labels
- Drag-and-drop file upload for MDX files
- Loading states and error handling

## Quality Standards
- All Go code must pass `go vet` and `golangci-lint` (if available)
- Frontend must build without errors
- Docker build must succeed
- Include a README.md with setup instructions

## Step-by-Step Execution Plan
1. Initialize Go module and install lib-x/mdx dependency
2. Create backend: models, database layer (SQLite), dictionary service
3. Create backend: API routes, auth middleware, handlers
4. Initialize frontend project with Vite
5. Build frontend pages: Login, Search, Dictionary Manager
6. Connect frontend to backend API
7. Create Dockerfiles for both services
8. Create docker-compose.yml
9. Test end-to-end: build, run, verify search works
10. Write README.md

## Constraints
- This is an ARM64 (aarch64) Linux system
- Use Go 1.21+ features
- Keep dependencies minimal
- Chinese dictionary content should render correctly (UTF-8 everywhere)
