# Hotclip Monorepo Plan

## Goal

Evolve hotclip from a single Go TUI into a multi-component monorepo that enables cross-device link sharing: capture links on Android, access them from the desktop TUI.

## Why a monorepo

- Shared Go types and API contract live in one place, no publishing overhead
- Solo-dev workflow: one repo, one PR, one CI pipeline
- Rejected alternatives: separate repos (too much cross-repo friction), git submodules (fragile, confusing)

---

## Target layout

```
hotclip/
├── cli/                    # existing Go TUI
│   ├── cmd/hotclip/        # main entrypoint (moved from root cmd/)
│   ├── internal/           # TUI internals (moved from root internal/)
│   ├── go.mod              # module: github.com/bridgkick/hotclip/cli
│   └── go.sum
├── server/                 # Go sync API server
│   ├── cmd/server/         # main entrypoint
│   ├── internal/           # handlers, auth, storage
│   ├── go.mod              # module: github.com/bridgkick/hotclip/server
│   └── go.sum
├── mobile/                 # Android app (Kotlin / Jetpack Compose)
│   ├── app/
│   │   └── src/main/
│   │       ├── java/ca/bridg/hotclip/
│   │       │   ├── MainActivity.kt
│   │       │   ├── ShareActivity.kt   # receives Android share-sheet intents
│   │       │   └── sync/              # sync client
│   │       └── res/
│   ├── build.gradle.kts
│   └── settings.gradle.kts
├── shared/                 # language-neutral API contract
│   └── schema.go           # canonical Link type, API request/response types
├── go.work                 # Go workspace linking cli + server (+ shared if Go)
├── README.md
└── PLAN.md                 # this file
```

---

## Step 1 — Restructure existing code under `cli/`

**Branch:** `monorepo` (already created off `main`)

### Actions

1. Create `cli/` directory.
2. Move `cmd/` → `cli/cmd/` and `internal/` → `cli/internal/`.
3. Create `cli/go.mod` with module path `github.com/bridgkick/hotclip/cli`, copying dependencies from the root `go.mod`.
4. Delete the root `go.mod` / `go.sum` (the workspace file replaces them at the root).
5. Create `go.work` at the repo root:
   ```
   go 1.25

   use ./cli
   ```
6. Update all internal import paths if needed (unlikely — they're already relative within the module).
7. Verify: `go build ./...` from `cli/` and `go work sync` from root both succeed.
8. Move `hotclip.exe` build target to `cli/`.

**Done when:** `go build github.com/bridgkick/hotclip/cli/cmd/hotclip` produces a working binary identical to today's.

---

## Step 2 — Define shared schema in `shared/`

### Actions

1. Create `shared/` with its own `go.mod`: module `github.com/bridgkick/hotclip/shared`.
2. Define the canonical `Link` type:
   ```go
   type Link struct {
       ID        string    `json:"id"`
       URL       string    `json:"url"`
       Title     string    `json:"title,omitempty"`
       Tags      []string  `json:"tags,omitempty"`
       CreatedAt time.Time `json:"created_at"`
       UpdatedAt time.Time `json:"updated_at"`
   }
   ```
3. Define API request/response types for sync operations (push batch, pull since cursor).
4. Add `./shared` to `go.work`.
5. Update `cli/internal/model` to import the shared `Link` type instead of its own.

**Done when:** `cli` compiles with the shared type and all existing tests pass.

---

## Step 3 — Build the sync server

### Actions

1. Scaffold `server/` with module `github.com/bridgkick/hotclip/server`.
2. Add `./server` to `go.work`.
3. Implement endpoints:
   | Method | Path | Description |
   |--------|------|-------------|
   | `POST` | `/auth/token` | Exchange device secret for JWT |
   | `POST` | `/links/push` | Client pushes a batch of new/updated links |
   | `GET`  | `/links/pull?since=<cursor>` | Server returns links updated after cursor |
   | `GET`  | `/health` | Liveness check |
4. Storage: SQLite via `modernc.org/sqlite` (zero-CGO, single file, trivially deployable).
5. Auth: per-device static tokens stored in a config file (no OAuth for now — revisit when multi-user).
6. Schema: use shared types from `github.com/bridgkick/hotclip/shared`.
7. Deployment target: single binary, run on a cheap VPS or home server.

**Done when:** `curl` can push a link and pull it back; server survives a restart with data intact.

---

## Step 4 — Add sync client to the CLI

### Actions

1. Add a `sync/` package under `cli/internal/` with a client wrapping the server API.
2. Config file (`~/.config/hotclip/config.toml` or similar) stores:
   - `server_url`
   - `device_token`
   - `last_sync_cursor`
3. Sync strategy: **pull on startup, push on add/delete** (no background daemon — TUI is short-lived).
4. Conflict resolution: last-write-wins on `updated_at`; soft-deletes (deleted flag) propagate.
5. Offline: all operations write to local JSON store first; sync is best-effort and non-blocking.
6. Add a status indicator in the TUI footer showing last sync time or "offline".

**Done when:** A link added on the desktop appears in the server's DB; a link pushed from another device appears in the TUI after restart.

---

## Step 5 — Scaffold the Android app

### Actions

1. Create `mobile/` as a standard Android Gradle project (Kotlin DSL).
2. Minimum SDK: API 26 (Android 8.0) — covers ~95% of active devices.
3. UI: Jetpack Compose, single-activity.
4. Key screens:
   - **Link list** — mirrors TUI view; tap to open in browser, long-press to copy URL.
   - **Share receiver** — `ShareActivity` handles `ACTION_SEND` intents from other apps; auto-captures URL and syncs immediately.
5. Sync client: Kotlin coroutines + Ktor (or Retrofit) hitting the same server API.
6. Auth: device token stored in Android Keystore / EncryptedSharedPreferences.
7. No local SQLite on mobile initially — fetch from server on open, cache in memory.

**Done when:** Sharing a URL from Chrome on Android causes it to appear in `GET /links/pull` and then in the desktop TUI on next start.

---

## Decisions deferred

| Topic | Deferred until |
|-------|----------------|
| Multi-user / accounts | After solo dogfooding proves the sync works |
| iOS app | After Android is stable |
| End-to-end encryption | After basic sync is working |
| Cloud hosting / CI/CD | After Step 3 is proven locally |
| Real-time push (WebSocket/SSE) | After polling proves too slow in practice |

---

## Branch strategy

- `main` — always builds; contains only the original TUI until the monorepo restructure is verified end-to-end
- `monorepo` — all restructure work; merge to `main` at the end of Step 1 once the CLI build is confirmed identical
- Feature branches off `monorepo` for Steps 2–5

---

## Current status

| Step | Status |
|------|--------|
| 1 — Restructure under `cli/` | Not started |
| 2 — Shared schema | Not started |
| 3 — Sync server | Not started |
| 4 — CLI sync client | Not started |
| 5 — Android app | Not started |
