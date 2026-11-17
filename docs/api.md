# Wherehouse API for other apps

Other apps, such as Warranteed, use Wherehouse through its JSON API with an API
token. An app can get one by [connecting](#connecting-an-app), or someone makes
one in **Settings → API tokens**. Send it with every request:

```
Authorization: Bearer wherehouse_…
```

A token acts as the person who made it, in the household they were in when
they made it. It never has admin or owner rights, and can't manage the
account: settings, API tokens, creating, switching or leaving households and
accepting invites answer 403 to a token. Delete it in Settings to stop it
working; it also stops when its owner leaves that household, or is removed
from the allowed groups at the identity provider (checked hourly).

A token from connecting an app can do less: only what keeping things in step
needs. Of what this page lists, that is:

- `GET /api/me`, `GET /api/household`, `GET /api/connect/households`, `GET /api/search`, `GET /api/changes`
- `GET /api/things`, `POST /api/things`, `GET`, `PATCH` and `DELETE /api/things/{id}`
- `POST /api/things/{id}/photos`, `GET` and `DELETE /api/photos/{id}`
- `GET /api/attachments/{id}/file`, `PATCH` and `DELETE /api/attachments/{id}`,
  `GET /api/paperless`, `POST /api/things/{id}/attachments/paperless`
- `POST /api/things/{id}/links`
- `PUT` and `DELETE /api/things/{id}/apps/{source}`, `GET /api/apps/{source}/{externalId}`
- `POST /api/things/{id}/maintenance`, `PUT` and `DELETE /api/maintenance/{id}`
- `PUT` and `DELETE /api/hook`, `DELETE /api/token`

Everything else, such as `PUT` of a whole thing, moving, copying or archiving,
kinds, labels, reports and the exports, answers 403
(`server.appNotAllowed`). A token made by hand can do all of it, and with it,
`GET /api/households` lists only the household the token is for.

Errors come as `{"error": "English text", "code": "server.…", "params": {…}}`
with a 4xx or 5xx status.

## Connecting an app

1. Make a random `code_verifier` (43 to 128 of `A–Z a–z 0–9 - . _ ~`) and keep
   it with your `state`. Send the person to
   `<Wherehouse>/connect?app=<your app's name>&redirect_uri=<your callback>&state=<random>&code_challenge=<challenge>&code_challenge_method=S256`,
   where the challenge is the SHA-256 of the verifier in base64url without
   padding ([PKCE](https://www.rfc-editor.org/rfc/rfc7636)).
2. Signed in, they pick a household and allow it. Their browser comes back to
   `redirect_uri` with `code` and your `state` (or `error=access_denied`).
3. From your server, `POST /connect/token` with
   `{"code": "…", "redirect_uri": "<the same>", "code_verifier": "…"}`.
   The answer is `{"token": "wherehouse_…", "household": {"id", "name"}}`.

A code works once, for 5 minutes, only with the same `redirect_uri` and only
with the verifier, so a code seen by someone else, e.g. in a log, is no use to
them.

When the person disconnects your app, `DELETE /api/token` with its token ends
it.

### Several households

A token is for one household. To connect several, add `&multiple=1` to the
`/connect` link, and optionally `&household=<id>` to tick one to begin with.
The person ticks one or more households; those your app (the same `app` name,
at the host of `redirect_uri`) is already connected to are shown as connected.
`POST /connect/token` then answers with a token for each:

```json
{"tokens": [{"household": {"id": 1, "name": "Home"}, "token": "wherehouse_…"}]}
```

Each token works like one from a single connection, with its own change feed
and hook. Connecting a household again replaces the app's earlier token for
it.

`GET /api/connect/households`, with any of your app's tokens, lists the
person's households: `[{"id", "name", "members", "connected"}]`, `connected`
saying whether your app has a token for it. Use it to offer connecting the
rest, with another `multiple=1` link. A token made by hand gets 403
(`server.connectedAppOnly`).

A token that answers 401 is gone: its household was deleted, the person left
it or was removed from it, or they deleted the token. Joining another
household by invite also deletes an empty household the person was alone in,
with its tokens.

### Being told about changes

With a token from connecting, `PUT /api/hook` with `{"url": "…"}` asks
Wherehouse to ping that address when something in the household changes;
the answer is `{"secret": "…"}`. The address must be on the host of the
`redirect_uri` the person approved, and is pinged only if it is a public
address, or a host the admin allowed in `WHEREHOUSE_HOOK_ALLOWED_HOSTS` (for an
app on the local network). A ping is a `POST` with
`{"event": "changed", "household": <id>, "at": "<time>"}`, signed in
`X-Wherehouse-Signature: sha256=<hex HMAC-SHA256 of the body with the secret>`.
It carries no data: read the change feed when it comes. Changes made with your
own token don't ping you, bursts of changes are gathered into one ping, and
pings aren't retried, so keep reading the feed now and then as well.
`DELETE /api/hook` stops them.

When the person creates a household, joins one or restores one from a
backup, every hook of theirs gets
`{"event": "households", "household": <id of the new one>, "at": "<time>"}`,
whichever household its token is for, so your app can offer to connect it.

## Things

Everything is a thing: a building, a room, a box, a drill. Any thing can
hold others. Its `kind` (`{id, key, name, icon, isPlace}`) says what it is;
things of a place kind, and things holding something, are shown as places.

| | |
| --- | --- |
| `GET /api/me` | The token's owner. Use it to check a token. |
| `GET /api/kinds` | The household's kinds. Built-in ones have a `key`: `building`, `room`, `furniture`, `box`, `thing`. |
| `GET /api/things` | Things, places first, then by name. Filters: `in=<id>` (directly inside), `inside=1` (with `in`: at any depth), `top=1` (in nothing), `unplaced=1` (in nothing and not places), `places=1` (only places, e.g. for a picker), `tag=<id>`, `archived=1`, `templates=1`, `recent=1` (newest change first), `limit=<n>`. Each has `path` (what it is in, outermost first), `contains` (how many things directly inside), `cover` (a photo ID), `label` and `tags`. |
| `GET /api/search?q=…` | Things matching every word of `q` in their name, details, tags, custom fields, label code, kind or what they are in. `archived=1` searches archived things. |
| `GET /api/things/{id}` | One thing with `photos`, `fields`, `attachments`, `maintenance`, `links` and `contents` (what is directly inside). |
| `POST /api/things` | Add a thing. Only `name` is needed; see the fields below. |
| `PATCH /api/things/{id}` | Change only the fields sent, e.g. `{"quantity": 2}`; everything else stays as it is at that moment. Use this from an app: a move or a sale made meanwhile isn't written back over. |
| `PUT /api/things/{id}` | Change it. Send every field you don't mean to clear; `tags` and `fields` left out stay as they are, a field set to `""` is cleared. Read it first and send it back changed. Not for tokens from connecting an app. |
| `POST /api/move` | Move things, with what is inside them: `{into, things: [ids]}`; `into` null is into nothing. |
| `POST /api/things/{id}/copy` | Copy a thing: `{sameParent: true}` beside itself, or `{parentId}` elsewhere; `{template: true}` makes a template; `photos: true` copies photos too. |
| `POST /api/things/{id}/archive`, `…/unarchive` | Archive a thing or bring it back. |
| `DELETE /api/things/{id}` | Delete a thing that holds nothing, with its photos and documents. |

Fields of a thing: `parentId`, `kindId` (0 for the built-in thing), `name`,
`description`, `quantity`, `manufacturer`, `model`, `serial`, `notes`;
purchase: `purchaseDate`, `purchasePrice` (of one piece), `purchaseFrom`,
`currency` (`""` is the household's); warranty: `warrantyUntil`,
`lifetimeWarranty`, `warrantyDetails`; `insured`; sale: `soldDate`,
`soldPrice`, `soldTo`, `soldNotes`; `tags` (names, added as needed);
`fields` (`{fieldId: value}`). Dates are `YYYY-MM-DD`.

## Photos, documents and links

| | |
| --- | --- |
| `POST /api/things/{id}/photos` | Add photos (multipart, one or more `file` fields). The first photo is the cover. |
| `GET /api/photos/{id}` | A photo; `?size=thumb` for a preview at most 600 px. |
| `POST /api/photos/{id}/cover`, `DELETE /api/photos/{id}` | Make a photo the cover, or delete it. |
| `POST /api/things/{id}/attachments` | Add a document (multipart: `file`, `kind` = `manual`, `receipt`, `warranty` or `other`). |
| `GET /api/attachments/{id}/file` | A document; `?download=1` to download it. |
| `GET /api/paperless` | The Paperless documents are linked from: `{url, publicUrl}`; 404 without one. Document IDs from another Paperless mean other documents. |
| `POST /api/things/{id}/attachments/paperless` | Add Paperless documents: `{documentIds, kind}`. Needs a Paperless token in the person's settings. |
| `POST /api/things/{id}/links` | Link the thing to a web address: `{url, title}`. Linking an address it already has changes the title. |
| `DELETE /api/links/{id}` | Remove a link. |

## Links from your app

An app links a thing to what it is there, at most once per thing, and can take
over the thing's purchase and warranty details.

| | |
| --- | --- |
| `PUT /api/things/{id}/apps/{source}` | Link the thing: `{externalId, url, title, data, purchase}`. `source` is your app's name in lowercase, e.g. `warranteed`; `data` is up to 4 KB of JSON for yourself. With `purchase` (the purchase and warranty fields), your app owns them: Wherehouse shows them read-only and keeps them when the thing is edited here, while insurance and a sale stay Wherehouse's. A thing already linked to another `externalId` of yours answers 409 (`server.appLinkTaken`): unlink it first. Linking the same `externalId` to another thing moves the link, and the purchase details of the thing it was on become Wherehouse's again. |
| `DELETE /api/things/{id}/apps/{source}` | Unlink it. Purchase details you owned stay and become Wherehouse's. |
| `GET /api/apps/{source}/{externalId}` | The thing linked to something of yours. |
| `GET /api/changes?since=<RFC 3339>` | Things changed since then, and IDs of those deleted; ask again from `until`. About 1000 things come at once: with `more: true` there are more, so ask again from `until` straight away. Use it to notice things sold, archived or moved. A thing whose label changed, or whose path changed because what it is in was moved or renamed, comes too. |

## Maintenance

| | |
| --- | --- |
| `POST /api/things/{id}/maintenance` | Log something done or to do: `{title, doneOn, nextDue, notes, cost}`, at least one date. |
| `PUT /api/maintenance/{id}`, `DELETE /api/maintenance/{id}` | Change or delete an entry. |
| `GET /api/maintenance/due?days=30` | What is due within that many days, and overdue. |

## Labels

| | |
| --- | --- |
| `GET /api/labels/{code}` | What a label is on: `{code, thing}`, or just `{code}` for a blank one. |
| `POST /api/labels` | Put a label on a thing: `{thingId}`, with `code` to use a given one. Without a code it keeps its label or gets a new one. Answers `{code}`. |

A label's QR code holds `<Wherehouse address>/l/<code>`.

## Value and export

| | |
| --- | --- |
| `GET /api/report` | Totals: `value` and `insured` by currency, counts, and value by place, kind and tag; `in=<id>` for what is inside one thing. |
| `GET /api/report/insured` | Insured things that weren't sold. |
| `GET /api/export.csv`, `GET /api/export.zip` | The household as a spreadsheet, or everything with its files. In the spreadsheet, text that would start a formula (`=`, `+`, `-`, `@`) gets an apostrophe in front. |
| `POST /api/restore` | Body: a ZIP from `export.zip`, up to 8 GB, unpacking to at most 8 GB of files. Makes a new household of it and switches to it; returns what came back. Signed-in session only. |
