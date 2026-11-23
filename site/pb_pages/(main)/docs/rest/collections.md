---
title: Collections API
description: Create, change, delete and import collections over HTTP - the endpoints the dashboard uses, for superusers.
---

# Collections API

The endpoints the dashboard uses to manage [collections](/docs/collections). All of them are for **superusers**. For
changes that travel with your code, prefer [migrations](/docs/migrations); these are for tools and scripts.

## List collections

`GET /api/collections`

Takes the [list parameters](/docs/rest#lists) (`page`, `perPage`, `sort`, `filter`, `fields`, `skipTotal`), filtering
on the collection's own properties (`name`, `type`, `system`).

```bash
curl --get http://127.0.0.1:8090/api/collections -H "Authorization: $TOKEN" \
  --data-urlencode "filter=type = 'auth'"
```

## View a collection

`GET /api/collections/{collection}`

The collection by name or id: its `fields`, rules, `indexes` and, for auth collections, their sign-in options.

## Create a collection

`POST /api/collections`

The body is the collection, the same shape `GET` returns:

```bash
curl -X POST http://127.0.0.1:8090/api/collections -H "Authorization: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "notes",
    "type": "base",
    "listRule": "owner = @request.auth.id",
    "fields": [
      {"type": "text", "name": "body", "required": true},
      {"type": "relation", "name": "owner", "collectionId": "_pb_users_auth_", "maxSelect": 1}
    ]
  }'
```

## Update a collection

`PATCH /api/collections/{collection}`

Send the properties you change. `fields` is the whole list: fields you leave out are removed (with their data), so send
every field, with its `id`, to keep it.

## Delete a collection

`DELETE /api/collections/{collection}`

Deletes it and all its records (answers `204`). A collection another collection's relation points to can't be deleted
until that relation is removed.

`DELETE /api/collections/{collection}/truncate` deletes all its records and keeps the collection (`204`).

## Import collections

`PUT /api/collections/import`

| Body | |
|---|---|
| `collections` | the collections, as exported (Settings → Export collections) |
| `deleteMissing` | `true` to delete collections (and their records) that aren't in the list |

Answers `204`. Collections are matched by id: existing ones are updated, new ones created.

## Helpers

| Endpoint | Answers |
|---|---|
| `GET /api/collections/meta/scaffolds` | a blank collection of each type (`base`, `auth`, `view`), with its default fields |
| `GET /api/collections/meta/oauth2-providers` | every OAuth2 provider PocketOne knows, with its display name and URLs |
| `POST /api/collections/meta/dry-run-view` | the fields a view collection's `{"query": "SELECT ..."}` would have, without saving it |

## Next

- [Collections and fields](/docs/collections): the collection shape and every field type.
- [Migrations](/docs/migrations): the same changes, versioned with your code.
