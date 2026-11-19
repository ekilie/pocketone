// Go's vanity import paths: PocketOne's Go modules are pocketone.dev/ext/<name> (and pocketone.dev/examples/pocketone,
// pocketone.dev/site/sync), served from the GitHub repo. `go get` asks for https://pocketone.dev/<import path>?go-get=1
// and reads the go-import meta tag, which maps the prefix pocketone.dev to the repo; each module's go.mod sits in the
// folder named by the rest of its path. Paths with no page reach here through +404.ejs, so every import path answers.
// See https://go.dev/ref/mod#vcs-find.
const REPO = 'https://github.com/pocketonehq/pocketone'

module.exports = (ctx, next) => {
  if (ctx.req.query('go-get') !== '1') return next()
  const meta = `pocketone.dev git ${REPO}`
  const source = `pocketone.dev ${REPO} ${REPO}/tree/main{/dir} ${REPO}/blob/main{/dir}/{file}#L{line}`
  return ctx.res.html(
    `<!doctype html><html><head>` +
      `<meta name="go-import" content="${meta}">` +
      `<meta name="go-source" content="${source}">` +
      `</head><body>go get pocketone.dev${ctx.req.url.pathname.replace(/\/+$/, '')}</body></html>`
  )
}
