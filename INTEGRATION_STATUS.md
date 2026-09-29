# EDMS integration status

Updated: 2026-09-22

## Implemented

- The Nuxt frontend uses the configured `NUXT_PUBLIC_API_BASE`; live route pages no longer claim seeded office records as operational data.
- The dashboard uses `GET /dashboard/overview` for recent records, submissions, visitor intake, and workflow totals.
- Favorites, incoming shares, reversible trash, global workflows, settings, departments, users, audit events, and storage health have persisted APIs and live frontend screens.
- Form drafts can be created, updated, published, and archived. Staff can load their submissions, pending approvals, submission detail, comments, attachments, and make approval decisions.
- Staff can create, list, inspect, revoke, and resend visitor requests.
- Staff now have a live approval inbox and visitor-access manager in the Forms workspace. Document detail has live sharing and signature-request controls.
- Visitors use an access code plus emailed OTP, receive a short-lived scoped session, upload supporting files, submit the form, and follow approval progress.
- Access codes, OTPs, and visitor session tokens are generated with `crypto/rand` and stored as hashes. OTP verification is constant-time; failed verification and repeated OTP requests are capped.
- AI has provider-neutral configuration and an application-owned interface. The included HTTP-compatible adapter supports hosted gateways and local inference servers without a vendor SDK. Classification, questions, summaries, extraction, and persisted job status are exposed under `/api/ai`.
- Document signing creates a persisted request through the configured adapter. HMAC-verified completion webhooks download the signed file, create a new immutable version, and advance an attached workflow step.
- Audit listing/export/retention, privacy export/deletion requests, legal holds, and email delivery status/retry APIs are present.
- OTP throttles use Valkey across application instances. Email delivery uses the Redis task queue when configured, including visitor access and OTP mail.
- Archive is reversible and keeps stored bytes. Permanent deletion is restricted to trashed documents and is refused while any active legal hold covers the document.
- Surreal Go ORM is updated to commit `b4651f508e9ea4e01e370a57ae88104409b52e91` (`v0.0.0-20260917093316-b4651f508e9e`) and all new schema changes use ORM migrations.
- New TypeScript/Vue logic uses arrow functions. Go uses idiomatic functions because Go has no arrow-function syntax.

## Deployment-dependent or deliberately unfinished

These are infrastructure or policy integrations, not hidden sample behavior:

1. **Malware scanning and quarantine.** Visitor uploads enforce session ownership and a 20 MB limit, but production should place a scanner/quarantine service in front of final storage.
2. **Provider delivery callbacks.** SMTP delivery records can be marked delivered, bounced, or failed, but production deployments still need to map their mail provider's signed callback format to that status API.
3. **Signing provider contract.** The webhook accepts common nested or flat completion payloads and verifies HMAC-SHA256. Confirm the exact event and signature headers emitted by the selected signing deployment before launch.
4. **AI protocol choice.** The bundled adapter expects HTTP `/chat/completions` and `/embeddings` semantics. Deployments using another protocol implement the small `ai.Provider` interface.
5. **Scheduled retention.** Staff can permanently delete trashed files and legal holds are enforced. Automatic organization-specific retention schedules still need a policy and scheduler.
6. **End-to-end infrastructure tests.** Go tests and the Nuxt production build pass. Full automation still needs test instances of SurrealDB, Redis/Valkey, SMTP capture, storage, an AI endpoint, and a signing endpoint.

## Visitor workflow

```text
Staff publishes form
        │
        ▼
Staff creates visitor request ──► access code + /visit link
        │
        ▼
Visitor enters email + code ──► rate-limited OTP delivery
        │
        ▼
Visitor verifies OTP ──► scoped temporary session
        │
        ├──► uploads supporting files
        ▼
Visitor submits form ──► approval steps ──► filed/complete
        │
        └──────────────── tracking view reflects route status
```

See [API_STATUS.md](API_STATUS.md) for the implemented endpoint inventory.
