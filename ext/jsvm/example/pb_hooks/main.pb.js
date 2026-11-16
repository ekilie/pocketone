// Top-level helpers and state are shared by every handler.
let visits = 0
const greet = (name) => "hello " + name

routerAdd("GET", "/hello/{name}", async (e) => {
	visits++
	// .async runs the Go call off the app's VM, so other requests run meanwhile
	const n = await e.app.countRecords.async("_superusers")
	return e.json(200, { message: greet(e.request.pathValue("name")), visits, superusers: n })
})
