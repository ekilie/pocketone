---
title: Logs API
description: List, filter and read the app's request and app logs, and count them by hour - for superusers.
---

# Logs API

Every request and every `$app.logger()` line is kept in the app's logs database for **Settings → Logs → Max days**
(5 by default). All endpoints are for **superusers**.

## List logs

`GET /api/logs`

Takes the [list parameters](/docs/rest#lists). Filter on `level` (`-4` debug, `0` info, `4` warn, `8` error),
`message`, `created` and any of `data`'s keys:

```bash
curl --get http://127.0.0.1:8090/api/logs -H "Authorization: $TOKEN" \
  --data-urlencode "filter=level >= 4 && data.url ~ '/api/collections/posts'" \
  --data-urlencode "sort=-created"
```

```json
{
  "page": 1, "perPage": 30, "totalItems": 1, "totalPages": 1,
  "items": [{
    "id": "lg6x2m9k4p1q8z0", "created": "2026-11-23 03:40:41.512Z", "level": 8,
    "message": "POST /api/collections/posts/records",
    "data": { "status": 400, "method": "POST", "url": "/api/collections/posts/records", "execTime": 1.2, "error": "Failed to create record." }
  }]
}
```

## View a log

`GET /api/logs/{id}`

## Count by hour

`GET /api/logs/stats`

How many log entries per hour, for a chart; takes `filter`:

```json
[{ "date": "2026-11-23 03:00:00.000Z", "total": 42 }]
```

## Delete all logs

`DELETE /api/logs` empties the logs database (answers `204`).

## Next

- [Custom routes](/docs/routes#middleware-of-your-own): write log lines from your own code with `$app.logger()`.
- [Settings API](/docs/rest/settings): how long logs are kept and at which level.
