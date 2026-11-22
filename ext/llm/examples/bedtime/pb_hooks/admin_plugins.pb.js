/// <reference path="../types.d.ts" />
// Settings → Bedtime Story in the dashboard: a superuser UI extension, registered the way PocketHost documents
// (pockethost.io/docs/admin-extensions). PocketBase serves pb_admin_ext/bedtime at /_/extensions/bedtime/ and loads its
// main.js into the dashboard through /_/extensions.js. Restart after changing this file; main.js changes show on reload.
$app.onServe().bindFunc((e) => {
  e.uiExtensions.push({
    name: "bedtime",
    fs: $os.dirFS(`${__hooks}/../pb_admin_ext/bedtime`),
  })
  e.next()
})
