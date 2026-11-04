# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- Office staff manage files, forms, routing, approvals, and access across departments.
- Office administrators configure departments, permissions, form templates, and visitor requests.
- Incoming visitors use a temporary access code and one-time password to complete a requested form, attach supporting information, and follow its progress without receiving a permanent staff account.

## Product Purpose

EDMS is an office file and process manager. It keeps a document's content, ownership, access, form data, and routing history together so staff and invited visitors can see what is required and where the work is in its lifecycle.

## Positioning

The product treats visitor intake as the beginning of a traceable office record rather than as a disconnected public form: the submitted information joins the same departmental approval and document workflow used by staff.

## Operating Context

Staff work from a shared web workspace organized by documents, managed folders, departments, forms, and approval routes. Visitors arrive from a staff-created link, verify short-lived credentials, submit required information, and return to a tracking view while the record moves between office desks.

## Capabilities and Constraints

- Go/Echo backend, Nuxt 4/Vue 3 frontend, SurrealDB persistence, and Surreal Go ORM migrations.
- Existing document upload, OCR, search, versioning, folders, forms, permissions, and routing behavior must remain intact.
- Visitor access must be time-limited, scoped to one request, and must not create a normal user account.
- AI integration must use a provider-neutral boundary and configuration. No vendor-specific product dependency is a product requirement.
- Email delivery depends on deployment SMTP configuration. Development may expose a safe local delivery path, but production must never return an OTP in an API response.
- The exact deployment environment, regulatory retention rules, and production AI provider remain open decisions.

## Brand Commitments

The working product name is EDMS. Product language should feel calm, precise, and operational rather than futuristic or promotional.

## Evidence on Hand

The repository contains working backend models and services for documents, forms, folders, routing, OCR, search, storage sync, access control, visitor intake, AI jobs, signing requests, audit operations, and privacy requests. Operational route pages use these APIs; old mock-only document presentation components remain isolated as legacy code and are not the source of live workspace totals.

## Product Principles

1. Make every record's next action and current owner obvious.
2. Give visitors the smallest safe amount of access needed to complete their request.
3. Keep transitions auditable from intake through filing.
4. Prefer live system state over decorative or fabricated dashboard statistics.
5. Keep infrastructure providers replaceable behind explicit interfaces.

## Accessibility & Inclusion

The web interface must support keyboard navigation, clear focus states, readable status labels that do not rely on color alone, responsive layouts, and reduced-motion preferences.
