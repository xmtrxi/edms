# EDMS integration status

Updated: 2026-10-26

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

## Accounts and security

- Sign-ins are revocable sessions: sign out, list devices, sign out elsewhere. Changing or resetting a password signs out other sessions.
- Password reset and email verification by single-use emailed links; verification can be required (`EDMS_REQUIRE_VERIFIED_EMAIL`).
- Two-step sign-in with authenticator apps (TOTP) and one-time recovery codes; secrets are encrypted at rest.
- Single sign-on with any OpenID Connect provider (PKCE, nonce, allowed domains, optional enforcement).
- Sign-in, reset and verification endpoints are rate limited across instances through Valkey.
- CORS follows `CORS_ORIGIN`; security headers, request size limits and server timeouts are on; `/metrics` and the task dashboard need monitoring credentials.

## Notifications

In-app inbox with live updates for shares, mentions, access requests and decisions, route steps and outcomes, form approvals and outcomes, signature requests and file-request uploads. "Email notifications", "Workflow digest" (daily summary of routine updates), "AI summaries" and "AI classification" preferences are enforced.

## Documents

- Uploads, new versions, visitor attachments and signed files from Documenso are scanned with ClamAV when `CLAMAV_ADDRESS` is set; infected files are refused and audited.
- PDF tools via Stirling-PDF: merge, compress, rotate, watermark, page numbers. Jobs are queued with retries and idempotency keys and store exactly one output version.
- Open in Microsoft 365 when storage is SharePoint: each person edits a working copy, checked in as a new version on demand and hourly.

## Operations

- Email carries proper headers and a Message-ID; Mailgun, Postmark, SendGrid or a generic signed webhook report delivery, bounces and complaints at `/api/email/events/:provider`.
- Administration → Integrations (self-hosted) shows stuck emails, extractions and PDF jobs, queue depth and dead-letter tasks, and retries them; stuck work is retried hourly.
- Prometheus metrics, OpenTelemetry tracing, backup/restore scripts and k6 load tests. See the README.

## Still deployment-dependent

1. **Signing provider contract.** Confirm the exact Documenso event and signature headers before launch.
2. **AI protocol choice.** The bundled adapter expects `/chat/completions` and `/embeddings`; other protocols implement `ai.Provider`.
3. **Stirling-PDF parameters.** Field names follow the Stirling-PDF v1 API; check them against the deployed version.
4. **Microsoft 365.** Needs SharePoint storage and a Graph app registration with Files.ReadWrite.All on the site.
5. **Translations.** The interface is English only.
6. **Timestamps.** The SurrealDB Go SDK stores times to the second, so records created within the same second have no defined order.

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
