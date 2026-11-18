# Cache: blocked work

Work on this extension that waits for something else to exist first. Each item names what it waits on; when that
lands, do the item and delete it here.

## A host command to empty the whole store

**Waits on:** an operator command surface on the multisite host that extensions can add commands to.

The extension's `Clear` empties every bucket of every instance (memory and disk tier); an instance
can only empty its own buckets (cache.clear/v1). When instances share the host's store (Pocker), the whole-store clear is the host's: wire `Clear`
into the host's operator commands (`cache clear`, with `--app <id>` and `--bucket <name>` to narrow it) once the host
has a place for extensions' commands. A plain `pocketone` command can't do it: it would run in its own process, with
its own store.
