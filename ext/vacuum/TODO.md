# Vacuum: blocked work

Work on this extension that waits for something else to exist first. Each item names what it waits on; when that
lands, do the item and delete it here.

## The host half

**Waits on:** `instance.maintenance/v1` on `main` (branch `e2e-multisite`).

A host extension that compacts every instance's databases on the operator's schedule, never an instance setting:

- Provide `vacuum/v1` for the host's instances (`vacuumv1.Contract.ProvideForInstances`), answering each instance's
  status (last run, bytes saved, next run) from the host's own record of its sweeps.
- The sweep: for each instance, check its databases read-only while it runs (free pages against `MinFreeRatio` /
  `MinFreeBytes`, a size cap, free disk space), and only for one worth it, stop it through
  `instance.maintenance/v1`, `VACUUM INTO` a new file, check its integrity, swap it in, and start it again if it was
  running. PocketHost's edge does all of this today (`packages/pockethost-go/edge/vacuum.go` in the PocketHost repo);
  port it here rather than writing it again.
- Configured by the operator only (a host section in `pocketone.yaml`).

## Choosing which instances

**Waits on:** the host half above, and pluggable lifecycle criteria for hosts (the planned `lifecycle.policy/v1`).

Which instances to compact (idle ones, ones the mothership marks `autoVacuum`) is a criterion the operator picks or
replaces, not code in this extension.

## The read-only status on each instance

**Waits on:** the host half above.

Register vacuum in every Pocker instance (a standard member, or the host's own instance extensions) so its Settings →
Backups section shows the host's status. `extkit.HostProvides` already keeps the setting out when the host provides
`vacuum/v1`; registering it before a host provider exists would give instances' admins a setting instead.

## PocketHost on this extension

**Waits on:** the host half above, and PocketHost moving to a PocketOne version that has it.

PocketHost's edge drops its own vacuumer for this extension's host half, keeping its own choices (the `autoVacuum`
flag, the Discord summary) as configuration and criteria.
