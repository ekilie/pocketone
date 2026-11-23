---
title: Custom routes
description: Add your own API endpoints to your app - path parameters, request bodies, JSON and HTML answers, auth requirements, middleware and errors - in TypeScript or Go.
---

# Custom routes

Your app already serves the [REST API](/docs/reference/rest) for its collections. A custom route adds an endpoint of
your own: a webhook receiver, a report, a search that joins three collections, anything that isn't plain record CRUD.

In JavaScript, add routes with `routerAdd` in a `pb_hooks/*.pb.ts` file. In Go, add them in an `onServe` hook.

## A first route

```ts
// pb_hooks/hello.pb.ts
routerAdd("GET", "/api/hello/{name}", (e) => {
  const name = e.request.pathValue("name")
  return e.json(200, { message: `Hello, ${name}!` })
})
```

```go
app.OnServe().BindFunc(func(se *core.ServeEvent) error {
	se.Router.GET("/api/hello/{name}", func(e *core.RequestEvent) error {
		name := e.Request.PathValue("name")
		return e.JSON(200, map[string]string{"message": "Hello, " + name + "!"})
	})
	return se.Next()
})
```

```bash
curl http://127.0.0.1:8090/api/hello/Ada
# {"message":"Hello, Ada!"}
```

- **Methods:** `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`, or `*` for any.
- **Paths:** `{name}` matches one segment, `{path...}` the rest of the path. API routes usually go under `/api/` (but
  not `/api/collections/`, which is the built-in API's), with a prefix of your own so they never meet a future one.
- A route's answer is whatever the handler returns: `e.json`, `e.string`, `e.html`, `e.redirect`, `e.noContent`,
  `e.blob`, or `e.stream`.

To serve HTML pages for people, rather than an API, [Pages](/docs/extensions/pages) is usually the better fit: a file
per route, with templates.

## Read the request

```ts
// pb_hooks/feedback.pb.ts
routerAdd("POST", "/api/feedback", (e) => {
  const data = new DynamicModel({ message: "", rating: 0 })
  e.bindBody(data) // JSON, form or multipart

  const info = e.requestInfo()
  const page = info.query["page"] // ?page=...

  if (data.message.trim() === "") {
    throw new BadRequestError("Write a message first.")
  }
  const record = new Record(e.app.findCollectionByNameOrId("feedback"))
  record.set("message", data.message)
  record.set("rating", data.rating)
  record.set("page", page)
  e.app.save(record)
  return e.json(201, { id: record.id })
})
```

```go
se.Router.POST("/api/feedback", func(e *core.RequestEvent) error {
	data := struct {
		Message string `json:"message" form:"message"`
		Rating  int    `json:"rating" form:"rating"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.BadRequestError("Couldn't read the body.", err)
	}
	page := e.Request.URL.Query().Get("page")

	if strings.TrimSpace(data.Message) == "" {
		return e.BadRequestError("Write a message first.", nil)
	}
	collection, err := e.App.FindCollectionByNameOrId("feedback")
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("message", data.Message)
	record.Set("rating", data.Rating)
	record.Set("page", page)
	if err := e.App.Save(record); err != nil {
		return err
	}
	return e.JSON(201, map[string]string{"id": record.Id})
})
```

- `e.requestInfo()` has the request's `query`, `headers`, `body` (already parsed) and `auth` in one object.
- `e.request` is the underlying HTTP request: `e.request.pathValue(...)`, `e.request.header.get(...)`,
  `e.request.method`.
- Saving through `e.app.save` runs your [hooks](/docs/hooks) and validation, as any other save does.

## Require sign-in

Routes are open to anyone unless you say otherwise. Add a middleware after the handler:

```ts
// pb_hooks/me.pb.ts
routerAdd("GET", "/api/me/summary", (e) => {
  const posts = e.app.countRecords("posts", $dbx.hashExp({ author: e.auth.id }))
  return e.json(200, { email: e.auth.email(), posts })
}, $apis.requireAuth())
```

```go
se.Router.GET("/api/me/summary", func(e *core.RequestEvent) error {
	posts, err := e.App.CountRecords("posts", dbx.HashExp{"author": e.Auth.Id})
	if err != nil {
		return err
	}
	return e.JSON(200, map[string]any{"email": e.Auth.Email(), "posts": posts})
}).Bind(apis.RequireAuth())
```

| Middleware | Lets through |
|---|---|
| `$apis.requireAuth()` | any signed-in record; `requireAuth("users")` only records of those collections |
| `$apis.requireSuperuserAuth()` | superusers |
| `$apis.requireSuperuserOrOwnerAuth("id")` | superusers, or the record whose id is the `{id}` path parameter |
| `$apis.requireGuestOnly()` | requests with no auth (a sign-up form) |
| `$apis.bodyLimit(bytes)` | bodies up to that size |
| `$apis.gzip()` | (compresses the answer) |

A client signs in through the [Authentication](/docs/authentication) API and sends its token in the `Authorization`
header; `e.auth` is then that record.

## Middleware of your own

A middleware is a function that takes the event and calls `e.next()` to go on. Pass it to one route, or to all of them
with `routerUse`:

```ts
// pb_hooks/timing.pb.ts
routerUse((e) => {
  const start = Date.now()
  e.next()
  e.app.logger().info("request", "path", e.request.url.path, "ms", Date.now() - start)
})
```

```go
se.Router.BindFunc(func(e *core.RequestEvent) error {
	start := time.Now()
	err := e.Next()
	e.App.Logger().Info("request", "path", e.Request.URL.Path, "ms", time.Since(start).Milliseconds())
	return err
})
```

In Go, `se.Router.Group("/api/shop")` makes a group whose routes share middleware.

## Errors

Throw one of the API errors and the client gets its status and message as JSON:

| Throw (JS) | Return (Go) | Status |
|---|---|---|
| `new BadRequestError(message, data?)` | `e.BadRequestError(message, err)` | 400 |
| `new UnauthorizedError(message)` | `e.UnauthorizedError(message, err)` | 401 |
| `new ForbiddenError(message)` | `e.ForbiddenError(message, err)` | 403 |
| `new NotFoundError(message)` | `e.NotFoundError(message, err)` | 404 |
| `new TooManyRequestsError(message)` | `e.TooManyRequestsError(message, err)` | 429 |
| `new ApiError(status, message, data?)` | `router.NewApiError(status, message, data)` | any |

```json
{ "status": 400, "message": "Write a message first.", "data": {} }
```

Any other error becomes a 400 with a generic message; its real text goes to the app's logs, so internals never reach
clients. A record that isn't found (`findRecordById` on a missing id) becomes a 404.

## Next

- [Hooks](/docs/hooks): run code on the built-in API's events instead.
- [Records](/docs/records): find and save records from a route.
- [Calling other services](/docs/reference/javascript/pocketbase#http): `$http.send` for webhooks out.
- [JavaScript API](/docs/reference/javascript/pocketbase): `routerAdd`, `$apis` and the request event's every method.
