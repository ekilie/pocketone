// GET /api/hello: a greeting from the server, plus every saved greeting (pb_migrations creates the collection).
routerAdd("GET", "/api/hello", (e) => {
  const rows = $app.findRecordsByFilter("greetings", "", "-created", 50, 0)
  return e.json(200, {
    message: "Hello from PocketOne hooks",
    greetings: rows.map((r) => r.get("text")),
  })
})
