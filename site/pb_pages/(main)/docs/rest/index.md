---
title: REST API
description: Every PocketOne app serves a REST API for its records, sign-in, files and realtime, plus admin endpoints for superusers. The base URL, auth header, errors, and the query parameters every list takes.
---

# REST API

Every PocketOne app serves a JSON API under `/api/`: its collections' records, signing in, files, realtime and, for
superusers, the admin endpoints the dashboard uses. The [JavaScript SDK](/docs/authentication#sign-in-from-a-client)
wraps it, but anything that speaks HTTP can use it.

| Area | What it covers |
|---|---|
| [Records](/docs/rest/records) | list, view, create, update and delete a collection's records |
| [Authentication](/docs/rest/authentication) | sign in, refresh, sign up flows, verify, reset, change email, impersonate |
| [Realtime](/docs/rest/realtime) | subscribe to record changes over server-sent events |
| [Files](/docs/rest/files) | download files and thumbnails, protected-file tokens |
| [Batch](/docs/rest/batch) | several record writes in one transaction |
| [Collections](/docs/rest/collections) | create and change collections (superusers) |
| [Settings](/docs/rest/settings) | the app's settings, test mail and S3 (superusers) |
| [Logs](/docs/rest/logs) | request and app logs (superusers) |
| [Crons](/docs/rest/crons) | list and run scheduled jobs (superusers) |
| [Backups](/docs/rest/backups) | make, download, upload and restore backups (superusers) |
| [SQL](/docs/rest/sql) | run a SQL query (superusers) |
| [Health](/docs/rest/health) | is the app up |
| [Extension APIs](/docs/reference/rest) | the endpoints PocketOne's extensions add, under `/api/_c/` |

## Requests

The base URL is your app's address: `https://myapp.example.com/api/...`, or `http://127.0.0.1:8090/api/...` locally.
Send JSON bodies with `Content-Type: application/json`, or `multipart/form-data` when you upload files.

## Auth

Sign in through an [Authentication](/docs/rest/authentication) endpoint and you get a token. Send it as the
`Authorization` header, with or without `Bearer `:

```bash
curl http://127.0.0.1:8090/api/collections/posts/records \
  -H "Authorization: eyJhbGciOiJIUzI1NiJ9..."
```

A request with no token is a guest. What each request may do is decided by the collection's [API
rules](/docs/api-rules); endpoints marked **superusers** need a token from the `_superusers` collection. An [API
token](/docs/features/tokens) works the same way for scripts and other apps.

## Lists

Every list endpoint takes the same query parameters:

| Parameter | Does | Example |
|---|---|---|
| `page` | the page, from 1 | `page=2` |
| `perPage` | items per page, up to 1000 (default 30) | `perPage=100` |
| `sort` | fields, `-` for descending, `@random` for random order | `sort=-created,title` |
| `filter` | a [filter](/docs/api-rules#the-filter-language) | `filter=(published=true && title~'hello')` |
| `expand` | relations to include, nested with dots, up to 6 levels | `expand=author,comments_via_post.author` |
| `fields` | only these fields in the answer; `*` for all at a level, `:excerpt(200,true)` to shorten | `fields=id,title,expand.author.name` |
| `skipTotal` | skip counting `totalItems` and `totalPages` (faster for big tables) | `skipTotal=1` |

A list answers:

```json
{
  "page": 1,
  "perPage": 30,
  "totalItems": 2,
  "totalPages": 1,
  "items": [{ "id": "...", "collectionName": "posts", "title": "Hello" }]
}
```

URL-encode `filter` and `sort` values in real requests (`curl --get --data-urlencode` does it for you).

## Errors

Errors answer with a status and a JSON body:

```json
{
  "status": 400,
  "message": "Failed to create record.",
  "data": {
    "title": { "code": "validation_required", "message": "Cannot be blank." }
  }
}
```

| Status | Means |
|---|---|
| 400 | the request is invalid; `data` names each field's problem |
| 401 | the endpoint needs auth and the request has none (or its token is invalid) |
| 403 | the request's auth may not do this |
| 404 | not found, or a rule hides it (so ids don't leak) |
| 429 | too many requests (rate limits, set in Settings) |

## Next

- [Records](/docs/rest/records): the endpoints most clients use.
- [API rules and filters](/docs/api-rules): who may call what.
- [Custom routes](/docs/routes): add endpoints of your own.
