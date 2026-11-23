---
title: Records API
description: List, view, create, update and delete a collection's records over HTTP, with filters, sorting, paging, expanded relations, field selection and file uploads.
---

# Records API

Each collection's records are at `/api/collections/{collection}/records`, where `{collection}` is the collection's
name or id. Who may call each endpoint is the collection's [API rule](/docs/api-rules) for that action; superusers may
call them all.

## List records

`GET /api/collections/{collection}/records`

Allowed by the **list** rule, which also filters what comes back: a client only ever sees the records the rule
matches.

| Query | |
|---|---|
| `page`, `perPage`, `sort`, `filter`, `expand`, `fields`, `skipTotal` | as for [every list](/docs/rest#lists) |

```bash
curl --get http://127.0.0.1:8090/api/collections/posts/records \
  -H "Authorization: $TOKEN" \
  --data-urlencode "filter=published = true" \
  --data-urlencode "sort=-created" \
  --data-urlencode "expand=author" \
  --data-urlencode "perPage=10"
```

```json
{
  "page": 1,
  "perPage": 10,
  "totalItems": 1,
  "totalPages": 1,
  "items": [
    {
      "collectionId": "pbc_1125843985",
      "collectionName": "posts",
      "id": "bxt6kzt400873g5",
      "title": "Hello World!",
      "slug": "hello-world",
      "published": true,
      "author": "0slkeutr0bxpw3v",
      "created": "2026-11-23 03:41:28.670Z",
      "updated": "2026-11-23 03:41:28.670Z",
      "expand": {
        "author": { "collectionName": "users", "id": "0slkeutr0bxpw3v", "name": "Ada" }
      }
    }
  ]
}
```

## View a record

`GET /api/collections/{collection}/records/{id}`

Allowed by the **view** rule; a record the rule doesn't match answers 404.

| Query | |
|---|---|
| `expand`, `fields` | as for lists |

```bash
curl http://127.0.0.1:8090/api/collections/posts/records/bxt6kzt400873g5?expand=author
```

## Create a record

`POST /api/collections/{collection}/records`

Allowed by the **create** rule, checked against the record the body would create. The body is the record's fields, as
JSON, or as `multipart/form-data` to upload files. Leave out `id` to have one made, or send your own (15 characters,
`a-z0-9`).

```bash
curl -X POST http://127.0.0.1:8090/api/collections/posts/records \
  -H "Authorization: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title": "Hello World!", "author": "0slkeutr0bxpw3v", "published": true}'
```

With a file:

```bash
curl -X POST http://127.0.0.1:8090/api/collections/posts/records \
  -H "Authorization: $TOKEN" \
  -F "title=With a cover" \
  -F "author=0slkeutr0bxpw3v" \
  -F "cover=@cover.jpg"
```

It answers the created record (`expand` and `fields` work here too). For an auth collection, send `password` and
`passwordConfirm` with the other fields: that's how sign-up works.

## Update a record

`PATCH /api/collections/{collection}/records/{id}`

Allowed by the **update** rule, checked against the record as it is now (and, with `@request.body`, what's being
sent). Send only the fields you change.

```bash
curl -X PATCH http://127.0.0.1:8090/api/collections/posts/records/bxt6kzt400873g5 \
  -H "Authorization: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title": "Hello again", "views+": 1, "tags+": ["news"]}'
```

Field modifiers change a value relative to what it is:

| Key | Does | Fields |
|---|---|---|
| `field+` | adds to a number; appends to a list | number, multi-select, multi-relation, multi-file |
| `+field` | prepends to a list | multi-select, multi-relation, multi-file |
| `field-` | subtracts from a number; removes items from a list | number, multi-select, multi-relation, multi-file |

To remove one file, send `cover-` with its file name; to remove all, send the field empty. To change an auth
record's password, send `oldPassword` (unless a superuser), `password` and `passwordConfirm`.

## Delete a record

`DELETE /api/collections/{collection}/records/{id}`

Allowed by the **delete** rule. Answers `204` with no body. Records pointing at it through a relation with **cascade
delete** are deleted too; otherwise a required relation pointing at it blocks the delete.

```bash
curl -X DELETE http://127.0.0.1:8090/api/collections/posts/records/bxt6kzt400873g5 \
  -H "Authorization: $TOKEN"
```

## Expand

`expand` includes related records in `expand` on each record, instead of only their ids:

- `expand=author`: the record a relation points to.
- `expand=author,tags`: several, comma-separated.
- `expand=author.team`: nested, up to 6 levels.
- `expand=comments_via_post`: backwards: the `comments` whose `post` relation points at this record.

Expanded records follow their own collection's **view** rule: one the client may not view is left out.

## Next

- [Authentication API](/docs/rest/authentication): get the token these examples send.
- [Realtime API](/docs/rest/realtime): hear about changes as they happen.
- [Batch API](/docs/rest/batch): several writes in one transaction.
- [Records](/docs/records): the same from your own code.
