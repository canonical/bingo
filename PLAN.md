# PLAN.md: Canonical Pastebin (bingo) - Go Re-implementation

## 1. Project Context & Vision
The goal of this project is to replace Canonical's outdated, legacy Pastebin application with a modern, maintainable, 12-Factor compliant workload named **bingo**. 

Initially, the Python/Django-based `dpaste` project was considered. However, it was rejected due to its reliance on outdated core frameworks, unmaintained dependencies, and server-side rendered template APIs. **While we are abandoning the `dpaste` codebase, its core business logic and feature set should be used as a conceptual reference for this re-implementation, which will be included in the workspace context**.

We are building this custom, highly performant Pastebin solution from scratch using **Go (Golang)**. It will expose a strict **JSON API** and generate its UI using Canonical's Pragma CLI.

## 2. Repository Structure & 12-Factor Strategy
This project utilizes a split-repository architecture:
1. **`bingo` (Current Repository):** The Go-based application workload.
2. **`bingo-k8s-operator` (Future Repository):** The Canonical 12-Factor OCI Charm used to deploy the application.

**Important Directive for the Agent:** Your scope is strictly limited to the `bingo` application workload. You must build this Go application to be 12-Factor friendly (stateless, configurable via environment variables, stdout logging). However, you do **not** need to generate the `charmcraft.yaml` or charm operator code in this repository. We will use the Canonical `12factor-charm` skill to package this app into the `bingo-k8s-operator` repository at a later date.

## 3. Technical Stack & Architecture
* **Backend Framework:** Go (using a modern router like `chi` or standard `net/http` for Go 1.22+).
* **API Paradigm:** JSON RESTful API. No server-side HTML rendering for the core service.
* **Frontend/UI:** Generated strictly using Canonical's **pragma CLI / MCP server** (https://github.com/canonical/pragma) to automatically adhere to Canonical's Vanilla framework styling and design standards.
* **Authentication (Agent Action Required):** You must evaluate and choose between **OIDC** (OpenID Connect) or **SPOE** (Single Point of Entry) for user authentication. You must provide a brief written rationale for your choice before implementing it in the middleware.
* **Database:** PostgreSQL. The application must treat the DB as an attached resource and remain entirely stateless.

## 4. Feature Requirements & Parity
The application must achieve feature parity with the legacy solution, referencing `dpaste` as the baseline:
* **Snippet Creation & Retrieval:** Submit raw text with syntax highlighting metadata, return a unique ID via JSON, and fetch snippet content via JSON API.
* **Unique ID Generation:** Implement collision-resistant, short-alphanumeric ID generation for snippets.
* **Expiration:** Snippets must support auto-deletion/expiration (e.g., never, a day, a week, a month, or a year). This should be handled via background workers or DB constraints.
* **Rate Limiting:** Implement robust API rate-limiting using modern Go concurrency patterns or maintained libraries (e.g., `x/time/rate`).

## 5. URL Schema & Routing Definitions
The routing configuration must explicitly support the multi-tenant nature of Canonical and Ubuntu services. The application must handle host-header-based styling/routing and preserve legacy pathing while hosting the new API:

### Legacy URL Routing (Must be supported for backwards compatibility)
* **Canonical Service:** `https://pastebin.canonical.com/p/{id}/`
* **Ubuntu Service:** `https://paste.ubuntu.com/p/{id}/`
* *Requirement:* A `GET` request to these legacy paths must correctly resolve and serve the snippet data via the Pragma UI layer, preserving the unique alphanumeric ID format (e.g., `v8Ck5mTVY4`, `Qd3trQDNqK`).

### Modern API Routing
* `POST /api/v1/snippet` -> Creates a snippet, returns JSON containing the new ID.
* `GET /api/v1/snippet/{id}` -> Returns raw snippet data and metadata as JSON.
*(Note: Ensure the API handles CORS and host validation appropriately for both target domains).*

## 6. Execution Steps for the AI Agent
Please execute the implementation sequentially. Complete one phase before moving to the next.

### Phase 1: Backend Initialization & API Design
* Initialize a new Go module named `bingo`.
* Scaffold the Go project structure (e.g., `/cmd/server`, `/internal/api`, `/internal/storage`).
* Define the JSON API routes (`POST /api/v1/snippet`, `GET /api/v1/snippet/{id}`).
* Implement the legacy path routing (`/p/{id}/`) to ensure incoming traffic from both `paste.ubuntu.com` and `pastebin.canonical.com` resolves correctly.
* Ensure the HTTP server binds to the `$PORT` environment variable.

### Phase 2: Core Logic, Auth & Storage
* Implement the PostgreSQL storage interface (connection via `$DATABASE_URL`).
* Write the `dpaste`-inspired logic for generating snippet IDs and syntax metadata handling.
* Implement the background worker or logic for snippet expiration.
* Implement API rate-limiting middleware.
* **Auth Choice:** State your choice between OIDC and SPOE with rationale, then implement the necessary authentication middleware.

### Phase 3: Frontend Generation (Pragma)
* Utilize the Canonical `pragma` CLI to scaffold the frontend layer within the repository.
* Ensure the generated frontend consumes the newly created Go JSON API and handles routing for the legacy `/p/{id}/` viewing paths.
* Validate that Canonical's styling conventions and standard UI components are preserved.

### Phase 4: 12-Factor Configuration Output
* Ensure all configuration variables (Database URI, Auth secrets, Rate limit thresholds) are strictly read from environment variables.
* Generate an `.env.example` file documenting every environment variable the `12factor-charm` will eventually need to map to Juju config options.
* Ensure all application logs stream unbuffered to `stdout`/`stderr`.

### Phase 5: Data Migration Tooling
* Write a Go script/CLI tool (e.g., inside `/cmd/migrate`) to handle data migration.
* The tool must be able to ingest a data export from the legacy schema, perform necessary data transformations (preserving the original snippet IDs to protect the legacy URLs), and safely insert it into the new `bingo` PostgreSQL schema to ensure zero data loss during cutover.