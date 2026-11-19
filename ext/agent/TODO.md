# Agent: blocked work

Work on this extension that waits for something else to exist first. Each item names what it waits on; when that
lands, do the item and delete it here.

## The host's model for containers

**Waits on:** an instance-scoped credential a host gives its Docker instances (one an instance can read, that the
host checks in `HostHandler`).

Instances on stock PocketBase images have no `$agent`; they could still call `/api/_c/agent/v1/complete` on their own
hostname with `$http.send`, answered by the host (`agentv1.W.HostHandler` in `h.InstanceRoutes`), once they have a
credential the host accepts. Their own-API settings then live with the instance record, not in the app.
