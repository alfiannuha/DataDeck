# DataDeck — Lightweight Web/PWA Database GUI
> **Product Requirements Document (PRD) & Technical Blueprint for Autonomous AI Agents**

---

## 1. Executive Summary & Vision

### 1.1 Problem Statement
Modern desktop database management clients (such as DBeaver, DataGrip, and Navicat) carry significant resource footprints (often requiring a Java Virtual Machine runtime, consuming 500 MB to 1.5 GB of idle RAM, and suffering from slow startup times). Web-based alternatives (such as CloudBeaver) often require complex containerized setups. Developers and data practitioners need an ultra-lightweight, zero-bloat, instantly responsive database client that can run locally, consume minimal memory (< 25 MB backend RAM), and run as an installable standalone Progressive Web App (PWA).

### 1.2 The Solution: DataDeck
**DataDeck** is a modern, modular, single-binary capable database management interface.
- **Backend:** A compiled Go daemon acting as a native TCP bridge, handling connection pooling, schema introspection, query execution, credential encryption, and local app state.
- **Frontend:** A Next.js (React 19 / TypeScript) interface styled with Tailwind CSS and Shadcn UI, featuring a CodeMirror 6 SQL editor with autocomplete and a TanStack Virtual data grid capable of rendering 100,000+ records at 60 FPS without DOM lag.
- **Distribution:** Can be executed as decoupled client/server services during development, deployed via Docker Compose, or compiled into a single static binary using Go's `embed.FS`.

---

## 2. High-Level Architecture & Data Flow

```text
+-------------------------------------------------------------------------+
|                              CLIENT TIER                                |
|    Next.js PWA / Standalone Desktop Window (Chrome / Edge / Safari)     |
|                                                                         |
|  +-------------------+  +--------------------+  +--------------------+  |
|  |  Schema Explorer  |  | CodeMirror 6 SQL   |  |  TanStack Virtual  |  |
|  |  (Tree Sidebar)   |  | Editor & Autocomp  |  |  High-Perf Grid    |  |
|  +-------------------+  +--------------------+  +--------------------+  |
+------------------------------------+------------------------------------+
                                     |
                                     | HTTP / REST + Server-Sent Events (SSE)
                                     v
+------------------------------------+------------------------------------+
|                         BACKEND SERVICE (GO DAEMON)                     |
|                                                                         |
|  +-------------------+  +--------------------+  +--------------------+  |
|  | Chi v5 HTTP Mux   |  | AES-256-GCM        |  | Dynamic Connection |  |
|  | + Swagger Docs    |  | Security Vault     |  | Pool Manager (LRU) |  |
|  +-------------------+  +--------------------+  +--------------------+  |
|                                     |                                   |
|             +-----------------------+-----------------------+           |
|             |                                               |           |
|             v                                               v           |
|  +----------------------+                      +---------------------+  |
|  | Local Storage        |                      | Database Drivers    |  |
|  | (Embedded SQLite)    |                      | - jackc/pgx/v5 (PG) |  |
|  | - Connection Profiles|                      | - go-sql-driver/my  |  |
|  | - History & Snippets |                      | - modernc/sqlite    |  |
|  +----------------------+                      +---------------------+  |
+------------------------------------+------------------------+-----------+
                                     |                        |
                                     | Direct TCP Dial        | SSH Bastion
                                     v                        v
                        +------------------------+  +---------------------+
                        | Target Local/Remote DB |  | Target Database in  |
                        | (PostgreSQL, MySQL)    |  | Private VPC / Cloud |
                        +------------------------+  +---------------------+
```

### 2.1 Execution Lifecycle
1. **Connection Initialization:** The frontend loads saved profiles (`GET /api/v1/connections`). Sensitive credentials are stored encrypted via AES-256-GCM in the local embedded SQLite database.
2. **Pool Management:** When a connection is activated, the Go backend checks its internal pool (`sync.Map`). If absent, it opens an active connection using the target driver, performs a 5-second timeout health ping, and stores the handle.
3. **Schema Tree Extraction:** The client requests schema metadata (`GET /api/v1/connections/{id}/schemas`). The backend queries system catalogs (e.g., PostgreSQL `information_schema` and `pg_catalog`) to return a JSON tree of schemas, tables, columns, data types, primary keys, and foreign keys.
4. **Query Execution & Dynamic Scan:** The user writes SQL in CodeMirror 6 and executes it via `Cmd+Enter`. The backend executes the query within a timeout-enforced context (`context.WithTimeout`), dynamically allocates typed scan pointers for arbitrary row structures, tracks execution duration in milliseconds, and streams or batches the JSON payload back to the client.
5. **Virtual Row Rendering:** TanStack Virtual calculates the viewport dimension and only mounts active table rows to the DOM.

---

## 3. Technology Stack & Decision Matrix

| Layer | Selected Tech | Version | Architectural Justification |
|---|---|---|---|
| **Frontend Framework** | Next.js (App Router) | 15.x | Deterministic routing, optimized production bundle, native PWA compatibility. |
| **Language (FE)** | TypeScript | 5.x | Strict type contracts for SQL schemas, API payloads, and query AST. |
| **UI Components** | Shadcn UI + Radix UI | Latest | Unstyled, accessible, easily tailored to native macOS desktop aesthetics. |
| **Styling** | Tailwind CSS | v4 | Zero-runtime CSS with comprehensive design token variables. |
| **Code Editor** | CodeMirror 6 | 6.x | Modular and ultra-lightweight (<300 KB bundle footprint vs Monaco's >5 MB). |
| **Data Grid Engine** | TanStack Table v8 + TanStack Virtual v3 | Latest | Virtualized row/column windowing; renders 100k+ records at 60 FPS without DOM memory bloat. |
| **State Management** | Zustand + TanStack Query v5 | Latest | In-memory query caching, state deduplication, and optimistic mutations. |
| **Backend Language** | Go (Golang) | 1.23+ | Native compilation, low RAM (<25 MB idle), concurrent goroutines. |
| **HTTP Router** | `github.com/go-chi/chi/v5` | v5.x | Zero-allocation, lightweight standard `net/http` idiomatic router. |
| **Target DB Drivers** | `jackc/pgx/v5`, `go-sql-driver/mysql`, `modernc.org/sqlite` | Latest | Pure Go implementations (modernc sqlite eliminates any CGO dependency). |
| **Local App Storage**| Embedded SQLite (`modernc.org/sqlite`) | Latest | Portable, zero-config local storage for history, favorites, and connections. |
| **Data Protection**  | `crypto/cipher` (AES-256-GCM) | Stdlib | Authenticated symmetric encryption for local password storage. |
| **Documentation**    | OpenAPI 3.0 / Swagger (`swaggo/swag`) | Latest | Declarative schema generation from Go struct tags. |

---

## 4. Internal Database Schema (App Storage / SQLite)

The backend creates and manages this SQLite database file automatically at `~/.datadeck/datadeck.db` (or `./data/datadeck.db`).

```sql
-- Connection Profiles Table
CREATE TABLE IF NOT EXISTS connection_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    driver TEXT NOT NULL CHECK(driver IN ('postgres', 'mysql', 'sqlite')),
    host TEXT,
    port INTEGER,
    database_name TEXT NOT NULL,
    username TEXT,
    encrypted_password TEXT,
    ssl_mode TEXT DEFAULT 'disable',
    ssh_enabled INTEGER DEFAULT 0,
    ssh_host TEXT,
    ssh_port INTEGER,
    ssh_username TEXT,
    ssh_encrypted_password TEXT,
    ssh_key_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Query Execution Audit & History Table
CREATE TABLE IF NOT EXISTS query_history (
    id TEXT PRIMARY KEY,
    connection_id TEXT NOT NULL,
    sql_text TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('SUCCESS', 'ERROR')),
    execution_time_ms INTEGER NOT NULL,
    rows_affected INTEGER DEFAULT 0,
    error_message TEXT,
    executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(connection_id) REFERENCES connection_profiles(id) ON DELETE CASCADE
);

-- Saved Queries / Snippets Table
CREATE TABLE IF NOT EXISTS saved_queries (
    id TEXT PRIMARY KEY,
    connection_id TEXT,
    title TEXT NOT NULL,
    sql_text TEXT NOT NULL,
    tags TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(connection_id) REFERENCES connection_profiles(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_query_history_conn ON query_history(connection_id, executed_at DESC);
CREATE INDEX IF NOT EXISTS idx_saved_queries_conn ON saved_queries(connection_id);
```

---

## 5. Monorepo Directory Layout

```text
datadeck/
├── .github/
│   └── workflows/
│       ├── test.yml
│       └── release.yml
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go                 # Go daemon entry point
│   ├── internal/
│   │   ├── api/
│   │   │   ├── handler/                # HTTP route controllers
│   │   │   │   ├── connection.go       # CRUD & ping test
│   │   │   │   ├── query.go            # SQL execution engine
│   │   │   │   └── schema.go           # Introspection controller
│   │   │   ├── middleware/             # Logging, CORS, Panic Recovery
│   │   │   │   ├── cors.go
│   │   │   │   └── logger.go
│   │   │   └── router.go               # Chi router assembly
│   │   ├── config/                     # Env var & runtime settings
│   │   │   └── config.go
│   │   ├── database/                   # Connection pool & driver managers
│   │   │   ├── manager.go              # Pool manager with sync.Map
│   │   │   ├── postgres.go             # PG system catalogs query
│   │   │   ├── mysql.go                # MySQL system catalogs query
│   │   │   └── sqlite.go               # SQLite pragma reader
│   │   ├── model/                      # Transfer objects and entities
│   │   │   ├── connection.go
│   │   │   ├── query.go
│   │   │   └── schema.go
│   │   ├── repository/                 # SQLite storage layer
│   │   │   ├── connection_repo.go
│   │   │   └── history_repo.go
│   │   └── security/                   # AES-256-GCM encryption
│   │       └── cipher.go
│   ├── docs/                           # Auto-generated Swagger files
│   │   ├── docs.go
│   │   ├── swagger.json
│   │   └── swagger.yaml
│   ├── go.mod
│   ├── go.sum
│   └── Makefile
├── frontend/
│   ├── public/
│   │   ├── manifest.json               # PWA configuration
│   │   ├── icons/                      # PWA application icons
│   │   └── favicon.ico
│   ├── src/
│   │   ├── app/
│   │   │   ├── layout.tsx              # Root HTML wrapper and providers
│   │   │   ├── page.tsx                # Main IDE workspace
│   │   │   └── globals.css             # Tailwind design tokens
│   │   ├── components/
│   │   │   ├── editor/                 # CodeMirror 6 components
│   │   │   │   ├── SqlEditor.tsx
│   │   │   │   └── extensions.ts
│   │   │   ├── grid/                   # TanStack virtualized table
│   │   │   │   ├── DataGrid.tsx
│   │   │   │   ├── GridHeader.tsx
│   │   │   │   └── GridCell.tsx
│   │   │   ├── sidebar/                # Tree explorer and connection list
│   │   │   │   ├── SchemaTree.tsx
│   │   │   │   ├── ConnectionList.tsx
│   │   │   │   └── NewConnectionModal.tsx
│   │   │   ├── ui/                     # Shadcn components (Button, Modal, etc.)
│   │   │   └── shared/                 # Topbar, Tabs, Statusbar
│   │   │       ├── StatusBar.tsx
│   │   │       └── TabsManager.tsx
│   │   ├── hooks/
│   │   │   ├── useConnection.ts
│   │   │   ├── useQueryExecution.ts
│   │   │   └── useSchemaTree.ts
│   │   ├── lib/
│   │   │   ├── api-client.ts           # Standard fetch client
│   │   │   └── utils.ts
│   │   ├── store/
│   │   │   ├── useWorkspaceStore.ts    # Tabs and editor state
│   │   │   └── useConnectionStore.ts   # Active connection state
│   │   └── types/
│   │       ├── api.ts
│   │       └── schema.ts
│   ├── package.json
│   ├── tsconfig.json
│   ├── tailwind.config.ts
│   └── next.config.ts
├── docker/
│   ├── Dockerfile.backend
│   ├── Dockerfile.frontend
│   └── docker-compose.yml
├── Makefile
└── README.md
```

---

## 6. API Specifications (OpenAPI 3.0 Standard)

Standard Envelope Contract:
```json
{
  "success": true,
  "data": {},
  "error": null,
  "meta": {}
}
```

### 6.1 Route Definitions
- `GET    /api/v1/health` — Daemon health status.
- `GET    /api/v1/connections` — Fetch all configured profiles (passwords masked).
- `POST   /api/v1/connections` — Persist new connection profile.
- `POST   /api/v1/connections/test` — Verify connection parameters prior to saving.
- `DELETE /api/v1/connections/{id}` — Terminate connection pool and remove record.
- `GET    /api/v1/connections/{id}/schemas` — Retrieve structural hierarchy (catalogs, tables, columns, relations).
- `POST   /api/v1/query/execute` — Synchronous raw SQL query execution.
- `GET    /api/v1/query/history` — Audit log of executed queries.
- `POST   /api/v1/queries/saved` — Save a query snippet.

### 6.2 Raw Query Execution Endpoint

**POST** `/api/v1/query/execute`

#### Request Payload
```json
{
  "connection_id": "9f1c7d2e-4a6b-4e89-b88a-d5f356bf7312",
  "sql": "SELECT id, full_name, email, metadata, created_at FROM users WHERE status = 'active' LIMIT 50;",
  "timeout_seconds": 30
}
```

#### Success Response (`200 OK`)
```json
{
  "success": true,
  "data": {
    "columns": [
      { "name": "id", "type": "INT8" },
      { "name": "full_name", "type": "VARCHAR" },
      { "name": "email", "type": "VARCHAR" },
      { "name": "metadata", "type": "JSONB" },
      { "name": "created_at", "type": "TIMESTAMPTZ" }
    ],
    "rows": [
      [101, "Jane Doe", "jane@example.com", { "role": "admin" }, "2026-09-25T14:32:00Z"],
      [102, "John Smith", "john@example.com", { "role": "editor" }, "2026-09-25T14:35:10Z"]
    ],
    "rows_affected": 2,
    "execution_time_ms": 12,
    "truncated": false
  },
  "error": null
}
```

#### Error Response (`400 Bad Request` or `500 Internal Error`)
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "SQL_SYNTAX_ERROR",
    "message": "syntax error at or near \"WHERRE\"",
    "position": 62
  }
}
```

---

## 7. Frontend Specification & Key Modules

### 7.1 Progressive Web App (PWA) Manifest
Located at `frontend/public/manifest.json`:
```json
{
  "name": "DataDeck Studio",
  "short_name": "DataDeck",
  "description": "High performance local-first Web/PWA Database GUI",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#09090b",
  "theme_color": "#09090b",
  "icons": [
    {
      "src": "/icons/icon-192x192.png",
      "sizes": "192x192",
      "type": "image/png"
    },
    {
      "src": "/icons/icon-512x512.png",
      "sizes": "512x512",
      "type": "image/png"
    }
  ]
}
```

### 7.2 Core User Interface Interactions
1. **Collapsible Schema Explorer:**
   - Hierarchy: Profile $\rightarrow$ Database $\rightarrow$ Schema $\rightarrow$ Table $\rightarrow$ Column / Index / Foreign Key.
   - Context Menu (Right Click): *Select Top 100*, *Copy DDL*, *Count Rows*, *Drop Table*.
2. **SQL Editor Tab Workspace:**
   - Tab manager with dirty-state indicator.
   - CodeMirror 6 with SQL dialect parsing, keyword autocomplete, and query block detection.
   - Keybindings: `Cmd+Enter` / `Ctrl+Enter` (Run selected or current statement), `Cmd+S` (Save snippet).
3. **TanStack Virtualized Grid:**
   - Dynamic viewport virtualization (`useVirtualizer`).
   - Copy cell on click, inline sorting, column resizing, and export to CSV/JSON.

---

## 8. Installation, Local Setup & Development

### 8.1 System Requirements
- Go 1.23 or newer
- Node.js 20 LTS or newer (with `npm` or `pnpm`)
- Accessible PostgreSQL or MySQL server (for testing)

### 8.2 Backend Setup
```bash
cd backend

# Download Go module dependencies
go mod tidy

# Install swaggo CLI and generate OpenAPI specification
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o docs

# Run the backend bridge locally (starts on 127.0.0.1:8080)
go run cmd/server/main.go
```

### 8.3 Frontend Setup
```bash
cd frontend

# Install npm packages
npm install

# Start development server
npm run dev
# Open http://localhost:3000 in your browser
```

---

## 9. Deployment Strategies

### 9.1 Strategy 1: Static Embed in Go Binary (Single Executable Distribution)
Build the Next.js frontend into a static export and compile it directly into the Go binary using `embed.FS`.

1. Configure `frontend/next.config.ts`:
   ```ts
   import type { NextConfig } from 'next';
   const nextConfig: NextConfig = {
     output: 'export',
   };
   export default nextConfig;
   ```
2. Build the frontend:
   ```bash
   cd frontend && npm run build
   # Outputs static files to frontend/out
   ```
3. Embed inside `backend/cmd/server/main.go`:
   ```go
   //go:embed all:../../../frontend/out
   var frontendDist embed.FS
   ```
4. Build the zero-dependency binary:
   ```bash
   cd backend
   CGO_ENABLED=0 go build -ldflags="-s -w" -o datadeck cmd/server/main.go
   ```

### 9.2 Strategy 2: Multi-Container Docker Deployment
Use `docker/docker-compose.yml`:
```yaml
version: '3.8'

services:
  backend:
    build:
      context: ./backend
      dockerfile: ../docker/Dockerfile.backend
    ports:
      - "8080:8080"
    volumes:
      - datadeck_storage:/data
    environment:
      - PORT=8080
      - STORAGE_PATH=/data/datadeck.db
      - ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
    restart: unless-stopped

  frontend:
    build:
      context: ./frontend
      dockerfile: ../docker/Dockerfile.frontend
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://localhost:8080
    depends_on:
      - backend
    restart: unless-stopped

volumes:
  datadeck_storage:
```

---

## 10. AI Agent Execution Roadmap & Prompts

When running an autonomous or semi-autonomous AI coding agent, execute tasks in the following sequential phases:

### Phase 1: Backend Storage, Security & Connection Pool
> **Agent Prompt:**  
> *"Create the Go backend module in `/backend`. Implement an embedded SQLite storage system using `modernc.org/sqlite` to persist connection profiles based on the schema in the PRD. Implement AES-256-GCM symmetric encryption for passwords using a 32-byte secret key. Build a `ConnectionManager` with an in-memory connection pool (`sync.Map` of `*sql.DB`) supporting PostgreSQL (`jackc/pgx/v5`) and MySQL (`go-sql-driver/mysql`). Expose `POST /api/v1/connections/test` and `POST /api/v1/connections` endpoints using `go-chi/chi/v5` with structured error handling."*

### Phase 2: Schema Introspection & Query Execution Engine
> **Agent Prompt:**  
> *"Implement the schema introspection module in Go for PostgreSQL and MySQL. Query `information_schema` to return a nested hierarchy of databases, schemas, tables, and columns with data types. Then, implement `POST /api/v1/query/execute` using dynamic `sql.Rows.Scan` over interface slices. Measure query execution duration in milliseconds, handle `NULL` values, serialize binary/UUID fields cleanly to JSON, and record every execution to `query_history` in the SQLite database."*

### Phase 3: Frontend Layout, Theme & CodeMirror 6 Editor
> **Agent Prompt:**  
> *"Set up a Next.js 15 App Router project in `/frontend` using TypeScript, Tailwind CSS, and Shadcn UI. Implement a dark-mode first layout with a collapsible sidebar, a top connection selector, a tabbed query workspace, and a bottom status bar. Integrate CodeMirror 6 with SQL syntax highlighting, auto-completion, and a `Cmd+Enter` shortcut to trigger query execution via TanStack Query."*

### Phase 4: TanStack Virtual Data Grid & End-to-End Monorepo Integration
> **Agent Prompt:**  
> *"Build the data grid component in the frontend using TanStack Table v8 and TanStack Virtual v3. Ensure it renders up to 50,000 rows with fixed column headers, column resizing, and copyable cells. Connect the UI to the Go backend API. Add `manifest.json` and icons in `/frontend/public` to enable desktop standalone PWA installation."*

---

## 11. Security & Edge-Case Constraints

1. **Loopback Binding by Default:** The Go daemon must default to listening on `127.0.0.1` rather than `0.0.0.0` to prevent unauthorized access across local area networks.
2. **Buffer Safety Limits:** Queries returning over 50 MB of raw payload data must be truncated, returning a `"truncated": true` flag to prevent out-of-memory errors on both client and server.
3. **Execution Context Deadlines:** Every SQL query executed through the backend must use `context.WithTimeout(ctx, duration)` (default: 30 seconds) to prevent hung queries from monopolizing connection pools.
4. **Precision Preservation:** Large numbers (such as 64-bit integers (`BIGINT`)) must be serialized as strings in JSON to avoid loss of precision in JavaScript's 53-bit integer limit.