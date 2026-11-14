### What's covered

Your database: records, collections, settings and superusers. **Not** uploaded files, `pb_hooks`, `pb_public` or the
logs database. Use PocketBase's backups for those.

History starts with the first change after PITR is turned on, and goes back as far as the retention allows (72 hours
by default).

### Restore from the dashboard

1. Open **Settings → Backups**. The **Point-in-time restore** section shows how far back you can go, how big the
   history is and where it's stored.
2. Under **Restore to**, pick a date and time, then click **Restore**.
3. The app keeps serving while the old database is rebuilt, then restarts with it.

Only superusers see this section.

### Undo a restore

The database you replaced is kept in `pb_data/litestream/before-restore.db`, and history keeps recording, so a restore
can be undone. After a restore, the section shows when you restored and to what time, with an **Undo this restore**
button.

### From the command line

```bash
pocketone pitr status
pocketone pitr restore --at 2026-11-11T14:05:00Z --to /tmp/then.db   # a copy, any time
pocketone pitr restore --at 2026-11-11T14:05:00Z                     # in place
```

`--at` takes an RFC 3339 time. With `--to`, you get a copy of the database as it was, to inspect or pull records
from; the app keeps running. Without `--to`, run it while the server is stopped: the restore is swapped in when it
next starts.

### What it costs

About 176 KiB of memory per serving app, and the storage for your history. Replication runs only while the app is
serving, never during CLI commands.
