# ONLYOFFICE Local Deploy + Callbacks and Microsoft 365 Open — Research & Plan

## 1. Goal & Repo Context

EDMS: Go 1.26 / Echo + SurrealDB + Valkey/Asynq + S3/Azure/local storage, Nuxt 4 frontend (`backend/internal/config/config.go:1`, `backend/internal/service/version.go:29`, `documents.md:315`). `documents.md:315-350` defines the ONLYOFFICE flow and `editor_sessions` models (not yet built — this plan fills `DOC-016..022`).

Constraints:
- Single-command local run for devs.
- Verifiable JWT + callback handling per `api.onlyoffice.com/docs/docs-api/usage-api/callback-handler/`.
- Path for orgs standardized on Microsoft 365 without duplicating EDMS storage.

---

## 2. Research: How ONLYOFFICE Document Server Works (verified 2026)

### 2.1 Runtime image & env

Image: `onlyoffice/documentserver` (Community 8.x, ~9.2 current). Sources: `github.com/ONLYOFFICE/Docker-DocumentServer`, `hub.docker.com/r/onlyoffice/documentserver`.

Key env vars:

```env
JWT_ENABLED=true                # default true since 7.2, auto-generated secret if unset
JWT_SECRET=<random 32+ chars>   # must match inbox/outbox/session on backend
JWT_HEADER=AuthorizationJwt     # default Authorization; example/repro uses AuthorizationJwt
JWT_IN_BODY=true|false          # validate token in POST body when header stripped by proxy
```

Do NOT mount `default.json` — override via `local.json` or env vars. Health:

- `curl -f http://onlyoffice-documentserver/healthcheck` -> `true` after ~60s warmup
- Inside container: `curl -f http://localhost:8000/info/info.json` (`healthcheck` stanza in `github/.../Docker-DocumentServer/docker-compose.yml` has `start_period: 60s`)

### 2.2 Request / browser flow

Browser -> `DocsAPI.DocEditor(placeholder, {document, editorConfig, token})` -> loads `https://ds-host/web-apps/apps/api/documents/api.js`.

- `document.url` — URL the DS container GETs to fetch the source file.
- `editorConfig.callbackUrl` — URL the DS container POSTs status updates to.

Biggest local pitfall: `localhost` inside the container != host. Fix with shared docker network + service names (`backend`, `onlyoffice-documentserver`) or `host-gateway`; never `127.0.0.1`.

### 2.3 Callback contract

`POST editorConfig.callbackUrl` with JWT in header (`JWT_HEADER`) or body. Body fields:

| `status` | Meaning | Has `url`? | Backend must do |
|----------|---------|------------|-----------------|
| 1 | editing / presence heartbeat | no | update `editor_sessions.last_callback_at`, participants; respond `{"error":0}` |
| 2 | ready for save (closed after changes, ~10s delay) | yes (`url`, `changesurl`, `history`) | `GET url` (JWT-signed, hashed like presigned S3, expires 900s via `storage.fs.urlExpires`), stream -> `VersionService.CreateVersion(source=onlyoffice_edit, idempotency=key)`, close session |
| 3 | save error | yes | log, mark error, optionally retry manual download |
| 4 | closed no changes | no | close session, no version |
| 6 | forcesave (Save button / timer / `forcesave` command) | yes, `forcesavetype 0/1/2/3` | same as 2 but *keep session open*, update `last_saved_version_id` |
| 7 | forcesave error | yes | log |

Notes:
- `status:2` uses `callbackUrl` of user who last edited; `status:6 forcesavetype=1` uses saver.
- Co-edit selection changed in 5.5 and 7.0 (last joiner / last tab).
- `key` (document key) must be stable per *version* (`documentId + versionId` hash), not per request, otherwise DS forks.

Security:
- Callback body is JWT-signed; `url` includes hashed signature, not plain open URL.
- Validate JWT, allowlist callback origin, enforce size limits and checksum.

References:
- `api.onlyoffice.com/docs/docs-api/usage-api/callback-handler/`
- `guides.onlyoffice.com/installation/docs-configure-jwt.aspx`
- Community threads on `postData ETIMEDOUT` / `Empty Callback=undefined` (host-gateway fix)

---

## 3. Proposed `docker-compose.onlyoffice.yml` (local)

Create alongside existing `docker-compose.docling.yml:1`. Extend / compose-overlap — do **not** replace the docling file.

```yaml
# docker-compose.onlyoffice.yml
services:
  backend:
    build:
      context: .
      dockerfile: backend/Dockerfile
    ports: ["8080:8080"]
    env_file: [backend/.env]
    environment:
      ONLYOFFICE_ENABLED: "true"
      ONLYOFFICE_URL: "http://localhost:8082"                     # browser-facing
      ONLYOFFICE_INTERNAL_URL: "http://onlyoffice-documentserver" # DS -> backend (docker DNS)
      ONLYOFFICE_JWT_SECRET: "${ONLYOFFICE_JWT_SECRET:-change-me-32-chars-min}"
      ONLYOFFICE_JWT_HEADER: "AuthorizationJwt"
      ONLYOFFICE_CALLBACK_BASE: "http://backend:8080"             # DS POSTs here
      ONLYOFFICE_CONTENT_BASE: "http://backend:8080"              # DS GETs document here
      APP_URL: "http://localhost:3000"
    depends_on:
      onlyoffice-documentserver:
        condition: service_healthy
      surrealdb: { condition: service_healthy }
      valkey: { condition: service_healthy }
    networks: [edms]

  onlyoffice-documentserver:
    image: onlyoffice/documentserver:8.3
    container_name: edms-onlyoffice
    ports: ["8082:80"]  # 8082 avoids clash with 5001 docling / 6000 gotenberg
    environment:
      JWT_ENABLED: "true"
      JWT_SECRET: "${ONLYOFFICE_JWT_SECRET:-change-me-32-chars-min}"
      JWT_HEADER: "AuthorizationJwt"
      JWT_IN_BODY: "true"
      WOPI_ENABLED: "false"
      USE_UNAUTHORIZED_STORAGE: "true" # allow http content/callback in dev
      ALLOW_PRIVATE_IPADDR: "true"
    volumes:
      - onlyoffice_data:/var/www/onlyoffice/Data
      - onlyoffice_log:/var/log/onlyoffice
      - onlyoffice_lib:/var/lib/onlyoffice
      - onlyoffice_appdata:/var/lib/onlyoffice/documentserver/App_Data/cache/files
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/healthcheck"]
      interval: 15s
      timeout: 5s
      retries: 20
      start_period: 60s
    restart: unless-stopped
    networks: [edms]

  frontend:
    build: { context: frontend, dockerfile: Dockerfile }
    ports: ["3000:3000"]
    environment:
      NUXT_PUBLIC_API_BASE: "http://localhost:8080/api"
      NUXT_PUBLIC_ONLYOFFICE_URL: "http://localhost:8082"
    depends_on: [backend]
    networks: [edms]

  # Reuse existing infra or declare minimal for standalone local:
  surrealdb:
    image: surrealdb/surrealdb:latest
    command: start --log trace --user root --pass root memory
    ports: ["8000:8000"]
    healthcheck: { test: ["CMD","curl","-f","http://localhost:8000/health"], interval: 5s, retries: 10 }
    networks: [edms]
  valkey:
    image: valkey/valkey:8
    ports: ["6379:6379"]
    healthcheck: { test: ["CMD","valkey-cli","ping"], interval: 5s, retries: 10 }
    networks: [edms]

networks: { edms: { driver: bridge } }
volumes: { onlyoffice_data: {}, onlyoffice_log: {}, onlyoffice_lib: {}, onlyoffice_appdata: {} }
```

**Why split URLs:** Browser needs `http://localhost:8082/web-apps/...` (public `DS_URL`). DS container needs `http://backend:8080/api/integrations/onlyoffice/callback/:sessionId` and `.../content/:token` — those must resolve inside the bridge network via service name `backend`, not via `host.docker.internal` on Linux. Add `extra_hosts: ["host.docker.internal:host-gateway"]` only if backend runs outside compose.

**`.env` additions** (`backend/.env.example:1` style):

```env
ONLYOFFICE_ENABLED=false
ONLYOFFICE_URL=http://localhost:8082
ONLYOFFICE_INTERNAL_URL=http://onlyoffice-documentserver
ONLYOFFICE_JWT_SECRET=<generate: openssl rand -hex 32>
ONLYOFFICE_JWT_HEADER=AuthorizationJwt
ONLYOFFICE_CALLBACK_BASE=http://backend:8080
```

Gate all editor endpoints behind `ONLYOFFICE_ENABLED` so prod without DS still builds.

**Validation (verifiable):**

```bash
ONLYOFFICE_JWT_SECRET=$(openssl rand -hex 32) docker compose -f docker-compose.onlyoffice.yml up -d
curl -f http://localhost:8082/healthcheck  # => true after 60-90s
curl -f http://localhost:8082/web-apps/apps/api/documents/api.js | head
# Create doc, open /documents/:id/edit, check DS logs:
docker logs edms-onlyoffice --tail 100 | grep postData
# Negative JWT test:
curl -X POST http://localhost:8080/api/integrations/onlyoffice/callback/fake \
  -H "AuthorizationJwt: bad" -d '{}'  # expect 401
```

---

## 4. Backend Implementation (matches `documents.md:315ff`)

### 4.1 Package layout

```
backend/internal/onlyoffice/   # new
  config.go     -> OnlyOfficeConfig{Enabled, PublicUrl, InternalUrl, JwtSecret, Header, CallbackBase}
  jwt.go        -> Sign(map) / Verify(token) via github.com/golang-jwt/jwt/v5 (already in go.mod)
  client.go     -> BuildEditorConfig(doc, session, user) => {document{fileType,key,title,url,permissions}, editorConfig{callbackUrl,mode,user{name,id}}, token}
  callback.go   -> Handle(status, key, url, users...) strict: verify JWT (header or body), download with timeout+size cap, checksum, idempotency via provider_events / editor_sessions.last_callback, then call version.go shared CreateVersion
  force_save.go -> POST /command with c=forcesave & key=...

backend/internal/database/
  model_editor_session.go + migration (documents.md:168)

backend/internal/handler/onlyoffice.go

backend/internal/config/config.go:11  # extend Config with OnlyOfficeConfig
```

### 4.2 Endpoints (`backend/internal/api/server.go:66` add group)

```
POST   /api/documents/:id/editor-sessions                          -> create/join (auth + Can(edit))
GET    /api/integrations/onlyoffice/content/:token                 -> streams current version file (short-lived JWT 5m, no auth cookie needed for DS)
POST   /api/integrations/onlyoffice/callback/:sessionId            -> public, JWT-only, idempotent
POST   /api/documents/:id/editor-sessions/:sid/force-save          -> authenticated forward to DS command service
GET    /api/documents/:id/editor-presence                          -> participants
```

### 4.3 Key logic

- `DocumentKey` = `sha1(documentID + ":" + baseVersionID)[:20]` stable for co-edit round; new key after `CreateVersion`.
- Short-lived content token: `jwt.Sign({documentId, versionId, exp:5m})` passed as `?token=` on `document.url`; handler verifies without session cookie.
- Callback download: `GET url` with `AuthorizationJwt` header forwarded; verify size vs `services.CoAuthoring.server.limits_tempfile_upload` analogue; hash and call `VersionService.CreateVersion` with `source=onlyoffice_edit`, actor = `users[0]`, contributors = session participants.
- Idempotency: `ProviderEvent {provider:"onlyoffice", external_id:key+status+url}` unique constraint.
- Always respond `{"error":0}` within ~2s even if async version queue still processing; DS retries on non-zero.

### 4.4 Frontend

- `app/components/OnlyOfficeEditor.vue` — loads `http://localhost:8082/web-apps/apps/api/documents/api.js` via `useScript`, instantiates `DocsAPI.DocEditor`.
- `app/composables/useOnlyOfficeEditor.ts` — fetch config/token, manage save states.
- Route `/documents/:id/edit`.
- `EditorPresence.vue` polls `/editor-presence`.

---

## 5. Microsoft 365: How to "Open in M365"

Three distinct levels — do **not** promise CSPP-free Office-for-web iframe without qualification.

### 5.1 A. Full iframe editing (WOPI host) — requires CSPP

Docs: `learn.microsoft.com/.../microsoft-365/cloud-storage-partner-program/online/` — cloud storage partners must join **CSPP** (Cloud Storage Partner Program); not open to end customers. Must implement 8 WOPI REST endpoints:

```
GET  /wopi/files/{id}            -> CheckFileInfo (BaseFileName, OwnerId, Size, Version, SupportsLocks, UserCanWrite, ...)
GET  /wopi/files/{id}/contents   -> GetFile
POST /wopi/files/{id}/contents   -> PutFile + X-WOPI-Lock handling
POST /wopi/files/{id}            -> Lock/Unlock/RefreshLock/UnlockAndRelock (via X-WOPI-Override)
```

Plus discovery (`GET https://onenote.officeapps.live.com/hosting/discovery` XML), host-page iframe wrapping `https://word-edit.officeapps.live.com/we/wordeditorframe.aspx?WOPISrc=...&access_token=...` (POST token, verify `proofKeys` via `X-WOPI-Proof`). Review cycle 4-5 weeks per environment even after code is done.

WOPI clients compatible: Microsoft 365, Collabora Online, ONLYOFFICE. Reference impls: `cs3org/wopiserver`, `petrsvihlik/WopiHost` (.NET), `damorris25/wopi` (Go + S3).

Verdict for EDMS: expose pluggable `WOPIProvider` behind `WOPI_ENABLED` but **do not** ship as default — heavyweight and gated by Microsoft allow-list.

### 5.2 B. Recommended for M365-standard orgs — SharePoint Embedded / Graph (no CSPP)

EDMS already has `backend/internal/storage/sharepoint/provider.go:1` and `msgraph-sdk-go` (in `backend/go.mod:16`). Use **SharePoint Embedded** (FileStorage API) — the CSPP-free successor. Store the doc in a SharePoint Embedded container (or customer's own SharePoint drive) and open via `DriveItem.webUrl`:

```http
GET https://graph.microsoft.com/v1.0/storage/fileStorage/containers/{containerId}/drive/root:/file.docx
=> { webUrl: "https://{host}/:w:r/contentstorage/.../doc2.aspx?sourcedoc={guid}&action=edit" }

GET /drive/items/{itemId}?select=webUrl
# frontend: window.open(webUrl + "?action=edit", "_blank")
```

Supports `action=view`, `action=edit`, `webDavUrl` for desktop. Gives full co-authoring / AutoSave / version history natively in M365 without building WOPI, and stays inside tenant compliance boundary. EDMS keeps metadata/permissions; file truth becomes the `DriveItem` when `storage_provider=sharepoint`. Sync back via Graph delta/webhooks -> `CreateVersion(source=sharepoint_sync)`.

Desktop handoff (zero backend work) fallback: Office URI schemes `ms-word:ofe|u|https://edms.example.com/api/documents/:id/content?token=...` — opens Word/Excel desktop if installed; requires public URL reachable by Office.

### 5.3 C. Self-host on-prem Office Online Server (OOS) — WOPI without CSPP

If org has volume-licensed Office, they can run **Office Online Server** on-prem and point WOPI at it. Same REST contract as A but no Microsoft allow-list. `petrsvihlik/WopiHost` matrix marks OOS as "Production without CSPP" vs M365-for-web which needs it. Note: OOS is in extended support / superseded by SharePoint Embedded — warn about deprecation.

### 5.4 Recommendation matrix

| Org profile | Editor to offer | Effort | Co-author |
|-------------|-----------------|--------|-----------|
| No M365 / privacy-sensitive | ONLYOFFICE (self-host) — default | 1 compose + 5 endpoints | yes (fast mode) |
| Has M365, wants browser editing, owns tenant | **SharePoint Embedded via Graph** (store DriveItem, open `webUrl`) | add container type + Graph upload/download + webhook | yes (native) |
| ISV wanting branded M365 iframe over EDMS-owned S3 | Full WOPI host (CSPP) | 8 endpoints + discovery + proof keys + CSPP review | yes |
| All | Office URI `ms-word:ofe|u|...` fallback | 1 line | no |

Implement as `backend/internal/editor/` interface:

```go
type Provider interface {
  GetConfig(ctx context.Context, doc *database.Document, user *database.User, sess *database.EditorSession) (EditorConfig, error)
  HandleCallback(ctx context.Context, req CallbackRequest) error
}
```

with `onlyoffice`, `wopi`, `graph` implementations; frontend `EditorProvider` switch.

---

## 6. Frontend & Routing Hooks

- `/documents/:id` -> `DocumentActionBar.vue` shows:
  - `Edit in ONLYOFFICE` if `document.edit` + `ONLYOFFICE_ENABLED`
  - `Open in Microsoft 365` if `storage_provider == sharepoint` or `SHAREPOINT_*` configured; else URI-scheme fallback
- `OnlyOfficeEditor.vue` mounts `DocsAPI.DocEditor`; `WopiEditor.vue` / iframe for Graph `webUrl` if chosen — both use `useDocumentWorkspace`.
- `EditorPresence.vue` polls `/editor-presence`.
- `VersionTimeline.vue` shows `source: onlyoffice_edit | sharepoint_sync | m365_wopi`.

---

## 7. Security / Ops Must-Haves

- JWT secret rotation via env, same value on DS and backend; `AuthorizationJwt` must be in `AllowHeaders` CORS.
- Callback URL allowlist: only `onlyoffice-documentserver` network may POST `/integrations/onlyoffice/callback/*` (no cookie auth).
- Short-lived content tokens (5m), optimistic concurrency on `expected_parent_version_id` per `documents.md:304`.
- Size / checksum limits, ClamAV later (`DOC-044`); `storage.fs.urlExpires 900s`.
- `provider_events (provider, external_id) UNIQUE` for idempotency.
- Monitoring: DS `/healthcheck`, Prometheus `wopi_*` counters, dead-letter queue.

---

## 8. Phased Tasks

**Phase 0 — compose & secret (1d):** add `docker-compose.onlyoffice.yml`, `.env.example` vars, `make onlyoffice-up`. Risk low.

**Phase 1 — DS plumbing (3-4d):** `backend/internal/onlyoffice/{config,jwt,client,callback}`, `model_editor_session`, migrations, `handler/onlyoffice.go`, content-token route (public). Test with `curl` health + mock `postData`. Gate via `ONLYOFFICE_ENABLED`.

**Phase 2 — Version integration (2d):** wire callback `status 2/6` -> shared `CreateVersion`, `status 1/4` presence, participants. Add `provider_events` idempotency.

**Phase 3 — Frontend (2d):** `OnlyOfficeEditor.vue`, `useOnlyOfficeEditor`, `/documents/:id/edit` page, presence indicator.

**Phase 4 — M365 Graph path (3d, parallel):** if `SHAREPOINT_*` set, add `GET /documents/:id/m365/open-url` returning `webUrl`/`webDavUrl` via Graph; `ms-word:ofe|u|` fallback composable `useM365Open`. Document CSPP vs Embedded decision in `docs/m365.md`.

**Phase 5 — Hardening (1d):** E2E co-edit (2 browsers, one doc), force-save, conflict test, load test.

---

## 9. Open Questions Before Build

1. M365 tenant model: own tenant (enables SharePoint Embedded) vs wanting EDMS-hosted Office without M365 licenses? Determines WOPI vs Graph.
2. Data residency: keep primary copy in S3/local when opening in M365 (requires sync) or let SharePoint become source of truth per folder?
3. On-prem OOS: any customer already running Office Online Server (no CSPP, but deprecated)?
4. DS edition: Community (free) vs Enterprise (forcesave/mobile) — license constraint?
5. Public URL: prod domain/TLS for DS and backend (`APP_URL` must be HTTPS for WOPI `WOPISrc`).

---

## 10. References

- ONLYOFFICE callback handler: `api.onlyoffice.com/docs/docs-api/usage-api/callback-handler/`
- ONLYOFFICE JWT: `guides.onlyoffice.com/installation/docs-configure-jwt.aspx`
- ONLYOFFICE Docker: `github.com/ONLYOFFICE/Docker-DocumentServer`
- WOPI / CSPP: `learn.microsoft.com/microsoft-365/cloud-storage-partner-program/online/`
- WOPI requirements: `learn.microsoft.com/.../online/wopi-requirements`
- Office URI schemes: `learn.microsoft.com/office/client-developer/office-uri-schemes`
- SharePoint Embedded launch: `learn.microsoft.com/sharepoint/dev/embedded/development/tutorials/launch-experience`
- Reference WOPI hosts: `cs3org/wopiserver`, `petrsvihlik/WopiHost`, `damorris25/wopi`, `ti777777/officeweb`
