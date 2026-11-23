---
title: Crons API
description: List the app's scheduled jobs and run one now - for superusers.
---

# Crons API

The app's scheduled jobs: its own (backups, cleanups, whose ids start with `__pb`) and the ones your code adds with
`cronAdd`. All endpoints are for **superusers**.

## List jobs

`GET /api/crons`

```json
[
  { "id": "__pbLogsCleanup__", "expression": "0 */6 * * *" },
  { "id": "nightlyReport", "expression": "0 6 * * *" }
]
```

## Run a job now

`POST /api/crons/{id}`

Starts the job at once, outside its schedule, and answers `204` without waiting for it to finish.

```bash
curl -X POST http://127.0.0.1:8090/api/crons/nightlyReport -H "Authorization: $TOKEN"
```

## Next

- [JavaScript API: `cronAdd`](/docs/reference/javascript/pocketbase#cronadd): add a job from a hook file.
