# Cache: blocked work

Work on this extension that waits for something else to exist first. Each item names what it waits on; when that
lands, do the item and delete it here.

## Write-through, so the cache survives restarts

**Waits on:** `cachev1.Item` / `cachev1.ItemBucket` and their support in this store on `main` (branch `cache-items`).

Today the disk tier is overflow only: a value reaches disk only when memory evicts it, the key index lives only in
memory, and the `p1cache` folder is wiped when the process starts. Write-through keeps the cache across restarts:

- An operator switch in the `cache` section (store-level, the host's only on a multisite host) turns it on; off stays
  today's behaviour.
- Every Set also queues its value's file on the IO goroutine, off the lock, as spills do now. An item is marshaled
  there with `MarshalCache`, once, and stays live in memory; memory eviction then only drops the in-memory copy,
  since the file is already there.
- The key index (space, key, blob file, expiry) is persisted too, so a restart can find what each key pointed at:
  an append-only log of sets and deletes, compacted now and then, replayed at start instead of `wipe`. Expired
  entries and files no key points at are dropped while replaying.
- After a restart every value starts on disk. A byte value reads back as today; an item reads back as bytes and is
  decoded by the caller's `Decoder` on its first `GetItem`, then held live again.
- Content-addressed blobs stay shared across keys and spaces on disk as in memory; the disk limit still applies, and
  what it evicts leaves the index too.
- A Set returns before its file is written (write-behind on the IO queue), so a crash can lose the last writes; that
  is within cache/v1's promise (a Get may miss), and a replay never returns a value that was deleted or replaced
  before the crash, because the log's order is the lock's order.

## Coalesced gets: decode from disk once per value in flight

**Waits on:** `cachev1.Item` / `cachev1.ItemBucket` and their support in this store on `main` (branch `cache-items`).

The store already reads a blob's file once at a time (`blob.loading`): concurrent Gets wait, then look again, and are
served from memory if the reader moved the blob back there. With items, the expensive part is the `Decoder`, and
looking again isn't enough:

- One read and one decode per blob in flight, whatever number of `GetItem` calls ask for it meanwhile: the first
  caller reads the file and decodes it; the others wait on it and get the same item (or the same error), not a
  re-read. This holds even when the item can't go back to memory (larger than the store's memory, or the store is
  full), where today each waiter would read the file again.
- A plain `Get` waiting on the same read gets the file's bytes (its own copy), not the decoded item.
- The shared result is checked against the index before it's handed out, as reads are now: a key deleted or replaced
  while the read was in flight misses instead of getting the old value.
- Callers that pass different decoders for the same blob (two types under one key, or one content blob shared by
  keys read as different types) don't share a decoded result: the in-flight decode is keyed by blob and decoder, and
  the bytes are read once for all of them.
- A content blob (Set as bytes) read through `GetItem` is decoded on every call today, since the item can't replace
  bytes other keys share. Keep that, but coalesce concurrent decodes of it the same way.
- Covered by a test with a blocking decoder: N concurrent `GetItem`s, one file read, one decode, N identical items.

## Limits per bucket, not for the whole store

**Waits on:** `cachev1.Item` / `cachev1.ItemBucket` and their support in this store on `main` (branch `cache-items`).

Today the store has the limits that matter (`max_memory`, `max_items`, `eviction`, `max_disk` for every bucket of
every app together), and a bucket's own caps only trim it further inside that pool, so one busy bucket evicts every
other bucket's values. Make each bucket's limits its own budget instead:

- A bucket's `max_memory`, `max_items` and `eviction` (from its code, then `defaults`, then `buckets.<name>`) decide
  alone what it keeps in memory; the store-wide `max_memory`, `max_items` and `eviction` keys go (greenfield: no
  shim).
- The disk tier goes per bucket too: a `max_disk` cap per bucket (and in `defaults`), so what memory evicts from a
  bucket spills within that bucket's disk budget, and evicting from it touches only that bucket.
- Every bucket needs a bound: `defaults` gets built-in values (today's 128MiB memory and 1GiB disk are the starting
  point to size down from), so a bucket whose code and config set nothing is still capped.
- Values stay stored once by content. A value shared by several buckets counts in full against each bucket that
  holds it (as a bucket's `max_memory` counts it today), and leaves memory or disk only when no bucket still wants
  it there.
- On a multisite host, each instance's buckets have their own budgets; an instance may set `defaults` and `buckets`
  for itself as now, under any ceiling the host sets for its instances.
