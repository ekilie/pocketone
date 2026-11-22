Source: https://pocketbase.io/docs/api-sql/

# API SQL

### Run raw SQL query

Executes a raw SQL query string *(used primarily for the "SQL Console" UI in PocketBase v0.39+)*.

Only superusers can perform this action.

**Be very careful when using this API!**

It is intended for one-off analytic queries, the occasional VACUUM/PRAGMA optimize or debug purposes and NOT as the primary interface for interacting with your PocketBase data because, depending on the query, the execution could break your application and may not be reversible!

JavaScript:

```js
import PocketBase from 'pocketbase';

const pb = new PocketBase('http://127.0.0.1:8090');

...

await pb.collection('_superusers').authWithPassword('test@example.com', '1234567890');

await pb.sql.run('SELECT count(*) FROM users');
```

Dart:

```dart
import 'package:pocketbase/pocketbase.dart';

final pb = PocketBase('http://127.0.0.1:8090');

...

await pb.collection('_superusers').authWithPassword('test@example.com', '1234567890');

await pb.sql.run('SELECT count(*) FROM users');
```

##### API details

`POST /api/sql`

**Body Parameters**

| Param | Type | Description |
|---|---|---|
| Required query | String | The SQL query to execute. Multiple inline SQL queries are supported but only the result of the last one is returned. |

**Responses**

**200**

```
{
    "execTime": 0,
    "affectedRows": 0,
    "columns": [
        {
            "name": "count(*)",
            "type": "",
            "nullable": true
        }
    ],
    "rows": [
        ["1"]
    ]
}
```

**400**

```
{
  "status": 400,
  "message": "Failed to execute query. Raw error: ...",
  "data": {}
}
```

**401**

```
{
  "status": 401,
  "message": "The request requires valid record authorization token.",
  "data": {}
}
```

**403**

```
{
  "status": 403,
  "message": "The authorized record is not allowed to perform this action.",
  "data": {}
}
```
