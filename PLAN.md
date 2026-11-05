# Wherehouse — plan

Self-hosted home inventory: what you own, where it is, and what's in that box.
A sibling to Warranteed, not part of it. Warranteed tracks purchases and warranties; Wherehouse tracks things and places.

## Goals

- Adding a thing takes seconds: a photo and a name. Everything else is optional.
- Finding a thing is instant: type "hdmi" and see the shelf and box it's in.
- Boxes have QR labels: scan one with your phone and see what's inside.
- It looks and feels like Warranteed: same stack, same design language, works on a phone.

## Non-goals (v1)

- Lending, bookings, check-in/out.
- Private places or things (everyone in the household sees everything).
- A native mobile app. The web app with the camera and a QR scanner is enough.
- Consumables and stock levels (Grocy's job).

## Stack

Same as Warranteed, so code and habits carry over:

- Go 1.27, `net/http`, SQLite (`modernc.org/sqlite`, no cgo), SQL migrations in `internal/db/migrations`.
- React 19 + Vite + Tailwind 4 + Radix + TanStack Query + react-router + i18next (English, Czech).
- Frontend embedded in the binary; one distroless Docker image; data in `/data`.
- OIDC sign-in only, groups for access and admin, dev login for `make dev`.
- Same `Makefile` targets: `binary`, `dev`, `web`, `build`, `test`, `fmt`, `image`.

Copied from Warranteed rather than shared at first: `auth`, `config`, `i18n`, `paperless`, `thumb`, the web UI primitives.
Extract a shared module only when keeping two copies in sync starts to hurt.

## Done

- Things that can hold other things, so a box is a thing; custom kinds with built-in or uploaded icons.
- Search as you type, QR labels and label sheets, scanning.
- Tags, custom fields, documents and Paperless, maintenance with reminders.
- Purchase, warranty, insurance and sale details; the Value page and insurance list.
- Several households with invites; copying and templates; CSV and ZIP export.
- API tokens, connecting other apps, app links, a change feed and pings. Warranteed uses all of these.

## Scope

The integration with Warranteed is complete. From here on it gets fixes only, no new sync features: keeping two apps in step both ways is where most bugs come from.

## Still open

- Screenshots in the README, and a first release.
- Paperless linking is tested against a fake only.

Later, maybe: AI suggestions from a photo (name, manufacturer, model) with Claude, as Warranteed does for receipts; lending.

## Open questions

- Should places have their own custom fields, or only things?
- Label sizes to support first (which sheets or label printer do you have?).
