# API status

All paths below are relative to `/api`.

## Visitor intake

- `POST /visitor-requests`
- `GET /visitor-requests`
- `GET /visitor-requests/:id`
- `POST /visitor-requests/:id/revoke`
- `POST /visitor-requests/:id/resend`
- `GET /visitor-requests/:id/attachments/:attachment_id/download`
- `POST /visitor/access/request`
- `POST /visitor/access/verify`
- `GET /visitor/portal`
- `POST /visitor/submit`
- `GET|POST /visitor/attachments`
- `DELETE /visitor/attachments/:id`

## Forms and review

- `GET|POST /forms`
- `PATCH /forms/:id`
- `POST /forms/:id/publish`
- `POST /forms/:id/archive`
- `POST /forms/:id/submissions`
- `GET /forms/submissions/mine`
- `GET /forms/submissions/pending`
- `GET /forms/submissions/:id`
- `GET|POST /forms/submissions/:id/comments`
- `POST /forms/submissions/:id/decision`

## File-manager workspace

- `GET /dashboard/overview`
- `GET /favorites`
- `PUT|DELETE /documents/:id/favorite`
- `GET /shared`
- `GET|POST /documents/:id/shares`
- `DELETE /documents/:id/shares/:share_id`
- `GET /trash`
- `POST /documents/:id/trash`
- `POST /documents/:id/restore`
- `DELETE /documents/:id/permanent` (trashed documents only; legal holds enforced)
- `GET /workflows`
- `GET|PUT /settings`
- `GET /integrations/storage`

## AI and signing

- `POST /ai/classify`
- `POST /ai/ask`
- `POST /ai/summarize`
- `POST /ai/extract`
- `GET /ai/jobs/:id`
- `POST /documents/:id/sign` when signing is enabled
- `POST /signing/webhook` when signing is enabled (HMAC verified; no staff session required)

## Governance and operations

- `GET /admin/audit-events`
- `GET /admin/audit-events/export`
- `DELETE /admin/audit-events?before=...`
- `GET|POST /admin/email-deliveries`
- `POST /admin/email-deliveries/:id/retry`
- `PATCH /admin/email-deliveries/:id/status`
- `GET /privacy/export`
- `POST /privacy/deletion-requests`
- `GET|POST /privacy/legal-holds`
- `POST /privacy/legal-holds/:id/release`

Existing document, folder, OCR, search, version, routing, category, user, role, and department APIs remain in place.
