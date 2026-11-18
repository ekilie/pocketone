// Package deploy deploys a project's code to a PocketBase remote: pb_hooks, pb_migrations, pb_pages, pb_public, packages and
// the production node_modules tree, as a hash diff (manifest → upload only missing files in gzipped PHB1 bundles →
// commit, swapped in safely).
//
// Client half: `pocketone deploy | dev | pull | logs | restart [remote]` (remotes from ext/remotes). It finds the
// remote's deploy/v1 (/api/_c/deploy/v1, the files) through remote/v1's whoami, and restarts and reads logs through
// remote/v1.
//
// Server half: the deploy API. [Register] serves it on a single app (superusers; API tokens with the deploy scope);
// [New] builds it for any [Backend] and [Instances] (PocketHost runs it on its multisite host with its own backend).
package deploy
