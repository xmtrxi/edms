# Documents, Folders and Integrations

## Goal

Build a simple document platform where:

- administrators create folders and subfolders;
- folders have colors, categories and inherited permissions;
- users upload documents only where they have permission;
- SharePoint folder structures remain recognizable;
- document edits always create attributable versions;
- several users can collaborate through ONLYOFFICE;
- documents can be routed for review, editing, approval and signing;
- Documenso produces signed document versions;
- Stirling PDF operations can be added later behind a small Go interface;
- OCR and search continue to run asynchronously.

The detailed architecture is available in [docs/DOCUMENT_PLATFORM_EXECUTION_PLAN.md](docs/DOCUMENT_PLATFORM_EXECUTION_PLAN.md). This file is the shorter implementation checklist and source-of-truth model guide.

## Keep the design simple

Use these rules throughout the implementation:

1. A document belongs to one folder through `documents.folder_id`.
2. Folder permissions are stored once and inherited. Do not copy them to every document.
3. Document grants exist only for exceptions.
4. Route access is temporary and separate from normal folder permissions.
5. Every content change creates an immutable document version.
6. External services never update document rows or storage directly. They return output to the shared version service.
7. Folder names are logical organization. Storage paths remain immutable, so folder moves do not copy S3 objects.
8. Long operations use queues and idempotency keys.
9. Backend authorization is mandatory even when the frontend hides an action.
10. Prefer small provider interfaces so ONLYOFFICE, Documenso and Stirling remain replaceable.

## Source of truth

| Concern | Source of truth |
| --- | --- |
| Folder hierarchy | `folders.parent_id` and `folders.ancestor_ids` |
| Document location | `documents.folder_id` |
| Folder access | `folder_grants` plus department membership |
| Document exception | `document_grants` |
| Temporary workflow access | `route_access_grants` |
| Current file | `documents.current_version_id` |
| File history | `document_versions` |
| Editing collaborators | `editor_sessions` and `editor_session_participants` |
| Signing state | `signing_requests`, recipients and provider events |
| External folder identity | `folders.external_id` and `folders.external_path` |
| Searchable OCR output | extraction artifacts and Meilisearch index |

## Required models

The Go structs belong in `backend/internal/database`. Each model should have a matching SurrealDB migration.

### Folder

```go
type Folder struct {
    ID                 RecordID  `json:"id,omitempty"`
    OwnerID            string    `json:"owner_id"`
    ParentID           string    `json:"parent_id,omitempty"`
    AncestorIDs        []string  `json:"ancestor_ids"`
    Depth              int       `json:"depth"`
    Name               string    `json:"name"`
    NormalizedName     string    `json:"normalized_name"`
    Description        string    `json:"description,omitempty"`
    Color              string    `json:"color,omitempty"`
    InheritPermissions bool      `json:"inherit_permissions"`
    PermissionVersion  int64     `json:"permission_version"`
    StorageProvider    string    `json:"storage_provider,omitempty"`
    ExternalID         string    `json:"external_id,omitempty"`
    ExternalPath       string    `json:"external_path,omitempty"`
    CreatedBy          string    `json:"created_by"`
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
}
```

`AncestorIDs` is stored from root to parent. It keeps tree and permission reads efficient without introducing a complex closure table. Moving a folder updates the moved subtree in one service operation.

### FolderGrant

```go
type FolderGrant struct {
    ID            RecordID  `json:"id,omitempty"`
    FolderID      string    `json:"folder_id"`
    PrincipalType string    `json:"principal_type"` // user or department
    PrincipalID   string    `json:"principal_id"`
    Role          string    `json:"role"`
    Allow         []string  `json:"allow,omitempty"`
    Deny          []string  `json:"deny,omitempty"`
    CreatedBy     string    `json:"created_by"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

Use a small role set:

| Role | Default capabilities |
| --- | --- |
| `manager` | view, download, upload, edit, comment, route, sign, share, move, delete, manage folder |
| `editor` | view, download, upload, edit, comment, create version |
| `contributor` | view, download, upload and comment |
| `viewer` | view and download |
| `uploader` | list folder and upload without browsing unrelated documents |

`Allow` and `Deny` are optional capability overrides. Keep custom overrides uncommon so access remains understandable.

### FolderCategory

```go
type FolderCategory struct {
    ID         RecordID  `json:"id,omitempty"`
    FolderID   string    `json:"folder_id"`
    CategoryID string    `json:"category_id"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### Document changes

Add this field to the existing `Document` model:

```go
FolderID string `json:"folder_id"`
```

Continue using the existing `OwnerID`, but folder authorization determines normal access. The owner remains an administrator-level fallback for their resource.

### DocumentGrant

Use only when one document needs access different from its folder.

```go
type DocumentGrant struct {
    ID            RecordID  `json:"id,omitempty"`
    DocumentID    string    `json:"document_id"`
    PrincipalType string    `json:"principal_type"`
    PrincipalID   string    `json:"principal_id"`
    Role          string    `json:"role"`
    Allow         []string  `json:"allow,omitempty"`
    Deny          []string  `json:"deny,omitempty"`
    CreatedBy     string    `json:"created_by"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

### RouteAccessGrant

```go
type RouteAccessGrant struct {
    ID             RecordID  `json:"id,omitempty"`
    DocumentID     string    `json:"document_id"`
    WorkflowID     string    `json:"workflow_id"`
    WorkflowStepID string    `json:"workflow_step_id"`
    PrincipalType  string    `json:"principal_type"`
    PrincipalID    string    `json:"principal_id"`
    Capabilities   []string  `json:"capabilities"`
    ExpiresAt      time.Time `json:"expires_at,omitempty"`
    RevokedAt      time.Time `json:"revoked_at,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
}
```

### EditorSession

```go
type EditorSession struct {
    ID                 RecordID  `json:"id,omitempty"`
    DocumentID         string    `json:"document_id"`
    BaseVersionID      string    `json:"base_version_id"`
    DocumentKey        string    `json:"document_key"`
    OpenedBy           string    `json:"opened_by"`
    Status             string    `json:"status"`
    LastCallbackAt     time.Time `json:"last_callback_at,omitempty"`
    LastSavedVersionID string    `json:"last_saved_version_id,omitempty"`
    CreatedAt          time.Time `json:"created_at"`
    ClosedAt           time.Time `json:"closed_at,omitempty"`
}

type EditorSessionParticipant struct {
    ID             RecordID  `json:"id,omitempty"`
    EditorSessionID string    `json:"editor_session_id"`
    UserID          string    `json:"user_id"`
    JoinedAt        time.Time `json:"joined_at"`
    LeftAt          time.Time `json:"left_at,omitempty"`
    LastActivityAt  time.Time `json:"last_activity_at"`
}
```

### SigningRequestRecipient

The existing `SigningRequest` remains, but `ProviderDocumentID` should hold the canonical Documenso v2 envelope ID.

```go
type SigningRequestRecipient struct {
    ID                RecordID  `json:"id,omitempty"`
    SigningRequestID  string    `json:"signing_request_id"`
    ProviderRecipient string    `json:"provider_recipient_id,omitempty"`
    UserID            string    `json:"user_id,omitempty"`
    Name              string    `json:"name"`
    Email             string    `json:"email"`
    Role              string    `json:"role"`
    Status            string    `json:"status"`
    SigningURL        string    `json:"signing_url,omitempty"`
    SignedAt          time.Time `json:"signed_at,omitempty"`
    UpdatedAt         time.Time `json:"updated_at"`
}
```

### ProviderEvent

```go
type ProviderEvent struct {
    ID             RecordID      `json:"id,omitempty"`
    Provider       string        `json:"provider"`
    ExternalID     string        `json:"external_id"`
    EventType      string        `json:"event_type"`
    ResourceID     string        `json:"resource_id"`
    Payload        map[string]any `json:"payload"`
    Status         string        `json:"status"`
    Error          string        `json:"error,omitempty"`
    ReceivedAt     time.Time     `json:"received_at"`
    ProcessedAt    time.Time     `json:"processed_at,omitempty"`
}
```

`provider + external_id` must be unique. This makes webhooks and callbacks idempotent.

## Database indexes

Add indexes for common lookups:

```text
folders(owner_id, parent_id, normalized_name) UNIQUE
folders(parent_id)
folders(external_id, storage_provider)
documents(folder_id, updated_at)
folder_grants(folder_id, principal_type, principal_id) UNIQUE
folder_categories(folder_id, category_id) UNIQUE
document_grants(document_id, principal_type, principal_id) UNIQUE
route_access_grants(document_id, principal_id, revoked_at)
editor_sessions(document_id, status)
editor_sessions(document_key)
editor_session_participants(editor_session_id, user_id) UNIQUE
signing_request_recipients(signing_request_id)
provider_events(provider, external_id) UNIQUE
```

## Permission resolution

Create `backend/internal/service/access.go`.

```go
Can(ctx, userID, capability string, resource ResourceRef) (bool, error)
Require(ctx, userID, capability string, resource ResourceRef) error
EffectiveFolderAccess(ctx, userID, folderID string) (AccessSummary, error)
```

Resolve access in this order:

1. system administrator;
2. owner;
3. explicit document deny or allow;
4. active route grant;
5. direct folder grant;
6. nearest inherited parent grant;
7. default deny.

Fetch all relevant ancestor grants in one query using `ancestor_ids`. Cache the result by `user_id`, `folder_id` and `permission_version`. Invalidate it when grants, folder parents or department memberships change.

Apply `AccessService` to:

- folder create, update, move and delete;
- document list, upload, move, read, download and archive;
- OCR output;
- search and search snippets;
- versions and restore;
- ONLYOFFICE editor sessions and callbacks;
- signing requests;
- routes and route actions.

## Shared version service

Keep the existing `DocumentVersion` model. Add one service operation used by every file-changing feature:

```go
CreateVersion(ctx, CreateVersionInput) (*database.DocumentVersion, error)
```

Required input:

- document ID;
- expected parent version ID;
- actor ID and contributors;
- source and change summary;
- workflow, step or signing references;
- filename, MIME type and file reader;
- idempotency key.

The service streams and hashes the file, writes an immutable object, creates the version, updates the document's current version atomically, writes audit events, and queues OCR/search indexing.

Use optimistic concurrency. If `expected_parent_version_id` is no longer current, return a conflict instead of replacing newer work.

Recommended storage layout:

```text
documents/{documentID}/versions/{version}/original.ext
documents/{documentID}/versions/{version}/ocr.json
documents/{documentID}/versions/{version}/extracted.md
```

## ONLYOFFICE flow

1. User selects **Edit document**.
2. Backend checks `document.edit`.
3. Backend creates or joins an editor session for the current version.
4. Backend returns a JWT-signed editor config and short-lived content URL.
5. Users opening the same version receive the same document key and collaborate in fast mode.
6. Status `1` callbacks update presence.
7. Status `4` closes without creating a version.
8. Status `6` handles an explicit/checkpoint save when content changed.
9. Status `2` downloads final output and calls `CreateVersion` with source `onlyoffice_edit`.
10. Session participants become version contributors.

Add package:

```text
backend/internal/onlyoffice/
├── client.go
├── config.go
├── jwt.go
├── callback.go
└── client_test.go
```

Add endpoints:

```text
POST /documents/:id/editor-sessions
GET  /integrations/onlyoffice/content/:token
POST /integrations/onlyoffice/callback/:sessionId
POST /documents/:id/editor-sessions/:sessionId/force-save
GET  /documents/:id/editor-presence
```

JWT validation, short-lived content tokens, callback URL allowlisting, size limits and checksum validation are mandatory.

## Documenso flow

Replace the existing custom Documenso v1 HTTP implementation with an adapter around `github.com/documenso/sdk-go`. Keep the local `signing.Provider` interface and pin the SDK version.

1. Check signing permission.
2. Pin the request to the current document version.
3. Prepare a PDF without changing the pinned version.
4. Create a Documenso v2 envelope.
5. Upload the PDF and create recipients/fields.
6. Distribute for signing.
7. Save envelope and recipient IDs.
8. Verify `X-Documenso-Secret` on webhooks using constant-time comparison.
9. Save webhook events idempotently and queue processing.
10. On completion, download the signed PDF.
11. Call `CreateVersion` with source `signature`.
12. Save provider audit evidence and complete the route's sign step.

Add endpoint:

```text
POST /webhooks/documenso
```

The webhook returns quickly; downloads and version creation happen in a queue. Add a periodic reconciliation job for signing requests that remain non-terminal.

## Stirling PDF flow

Wait for the dedicated package, but define this boundary now:

```go
type PDFProvider interface {
    Merge(ctx context.Context, files []InputFile, options MergeOptions) (Result, error)
    Convert(ctx context.Context, file InputFile, target Format) (Result, error)
}
```

Merge and conversion run as queued jobs. Every successful result calls `CreateVersion` with source `stirling_pdf` or `conversion`.

## Routing flow

Keep the existing workflow models and add integration orchestration:

- `review` grants temporary view/comment access;
- `edit` grants temporary edit access and opens ONLYOFFICE;
- `approve` records the decision against an exact version;
- `sign` creates a Documenso request and waits for completion;
- `acknowledge` records receipt;
- `file` completes final filing.

When a step activates, create a `RouteAccessGrant`. When it completes or is reassigned, revoke that grant. Folder permissions must not be modified by routing.

Add an inbox:

```text
GET  /inbox/routes
POST /routes/:id/steps/:stepId/claim
POST /routes/:id/steps/:stepId/decisions
GET  /routes/:id/activity
```

## Frontend work

Keep pages thin and move behavior into focused components/composables.

### Routes

```text
/admin/folders
/documents
/documents/:id
/documents/:id/edit
/inbox
/signing/:requestId
```

### Components

- `FolderTree.vue`
- `FolderDetailsPanel.vue`
- `FolderPermissionEditor.vue`
- `PrincipalPicker.vue`
- `DocumentActionBar.vue`
- `OnlyOfficeEditor.vue`
- `EditorPresence.vue`
- `VersionTimeline.vue`
- `RouteBuilder.vue`
- `RouteTimeline.vue`
- `SigningWizard.vue`
- `IntegrationJobStatus.vue`

### Composables

- `useFolderAccess.ts`
- `useDocumentWorkspace.ts`
- `useOnlyOfficeEditor.ts`
- `useRouteInbox.ts`

The permission UI must explain direct and inherited access. User and department pickers replace raw database ID inputs. The editor shows saving, saved, offline and conflict states. Signing progress shows every recipient. Version history shows source, author, contributors and time.

## Ordered implementation tasks

### Phase 1 — Real folders and permissions

- [ ] `DOC-001` Add folder, grant, category and document-folder models.
- [ ] `DOC-002` Add migrations and indexes.
- [ ] `DOC-003` Add `folder_id` to documents.
- [ ] `DOC-004` Implement folder create, update, move, tree and delete services.
- [ ] `DOC-005` Implement `AccessService` and permission caching.
- [ ] `DOC-006` Apply access checks to documents, uploads, downloads, OCR and search.
- [ ] `DOC-007` Migrate virtual folder metadata into real folders.
- [ ] `DOC-008` Build the admin folder and permission UI.
- [ ] `DOC-009` Update upload and document browsing to use folder IDs.
- [ ] `DOC-010` Add permission and migration tests.

Done when an admin can create an empty folder, assign people/departments, and inherited permissions control every backend operation.

### Phase 2 — Version foundation

- [ ] `DOC-011` Implement the shared `CreateVersion` operation.
- [ ] `DOC-012` Add idempotency and optimistic concurrency.
- [ ] `DOC-013` Move upload, restore and sync version writes into the shared service.
- [ ] `DOC-014` Queue OCR and indexing after every new current version.
- [ ] `DOC-015` Expand version contributor and audit tests.

Done when every content change creates one immutable, attributable version and concurrent updates cannot overwrite each other.

### Phase 3 — ONLYOFFICE

- [ ] `DOC-016` Add ONLYOFFICE configuration and Docker deployment.
- [ ] `DOC-017` Add typed config, JWT and callback package.
- [ ] `DOC-018` Add editor session and participant models/migrations.
- [ ] `DOC-019` Add editor config, content, callback and force-save endpoints.
- [ ] `DOC-020` Save outputs through `CreateVersion`.
- [ ] `DOC-021` Build the editor page, presence and save-state UI.
- [ ] `DOC-022` Test collaboration, permissions, callbacks and recovery.

Done when two authorized users collaborate on one version and one attributable saved version appears in EDMS.

### Phase 4 — Routing

- [ ] `DOC-023` Add temporary route access grants.
- [ ] `DOC-024` Add assigned-user/department inbox and claim APIs.
- [ ] `DOC-025` Connect edit steps to ONLYOFFICE.
- [ ] `DOC-026` Require new versions after return-for-changes.
- [ ] `DOC-027` Build route builder, timeline and inbox UI.
- [ ] `DOC-028` Add notifications, due dates and route audit tests.

Done when a non-owner receives a task, gets only temporary required access, acts on a pinned version, and loses access after completion.

### Phase 5 — Documenso

- [ ] `DOC-029` Pin `documenso/sdk-go` and replace the v1 client internals.
- [ ] `DOC-030` Add recipient and provider-event models/migrations.
- [ ] `DOC-031` Add v2 envelope create/upload/distribute flow.
- [ ] `DOC-032` Add verified, idempotent webhook handling.
- [ ] `DOC-033` Add completion download and signed version creation.
- [ ] `DOC-034` Connect sign route steps.
- [ ] `DOC-035` Build signing wizard and recipient progress UI.
- [ ] `DOC-036` Add reconciliation and integration tests.

Done when a pinned PDF is signed by several recipients and the completed PDF plus evidence becomes a new immutable version.

### Phase 6 — Stirling PDF

- [ ] `DOC-037` Add the supplied Go package behind `PDFProvider`.
- [ ] `DOC-038` Add queued merge/conversion jobs and retries.
- [ ] `DOC-039` Save outputs through `CreateVersion`.
- [ ] `DOC-040` Build PDF tools and job-progress UI.
- [ ] `DOC-041` Add validation, retry and idempotency tests.

Done when PDF jobs can be retried safely and create exactly one traceable output version.

### Phase 7 — Scale and hardening

- [ ] `DOC-042` Add SSE or WebSocket updates for jobs, routes and editor presence.
- [ ] `DOC-043` Add batched search authorization and permission-cache metrics.
- [ ] `DOC-044` Add malware scanning and external-output validation.
- [ ] `DOC-045` Add dead-letter queues and integration reconciliation dashboards.
- [ ] `DOC-046` Load-test deep folder trees, ACLs, callbacks and large documents.

## Recommended package layout

```text
backend/internal/
├── database/
│   ├── model_folder.go
│   ├── model_folder_grant.go
│   ├── model_document_grant.go
│   ├── model_editor_session.go
│   └── model_provider_event.go
├── service/
│   ├── access.go
│   ├── folder.go
│   ├── version.go
│   ├── routing.go
│   └── signing.go
├── handler/
│   ├── folders.go
│   ├── onlyoffice.go
│   └── webhooks.go
├── onlyoffice/
├── signing/
└── pdf/
```

## Final acceptance checklist

- [ ] Empty folders persist without documents.
- [ ] Folder moves and renames do not move S3 objects.
- [ ] Permission inheritance and revocation are immediate.
- [ ] Search never exposes unauthorized names or snippets.
- [ ] SharePoint paths map to real folders.
- [ ] Uploads require folder permission.
- [ ] Every edit/conversion/signature creates a version.
- [ ] Version history shows who, when, source and contributors.
- [ ] ONLYOFFICE supports simultaneous authorized editors.
- [ ] Routing grants and revokes temporary access correctly.
- [ ] Signing is tied to an exact version.
- [ ] Webhooks and callbacks are verified and idempotent.
- [ ] OCR and Meilisearch process the current version asynchronously.
