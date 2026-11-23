---
title: Hooks
description: Run your own code when something happens in your app - before or after a record is saved, on an API request, when mail is sent, when the app starts. The hook model, written in TypeScript or Go.
---

# Hooks

A hook is a function your app calls when something happens: a record is about to be created, a request asks for a
list, an email is about to go out, the app starts. Your function sees the event, can change it or stop it, and then
lets it go on. Hooks are how you add rules, side effects and integrations without changing PocketOne itself.

In JavaScript, hooks live in `pb_hooks/*.pb.ts` files, which PocketOne loads at start and reloads when they change. In
Go, you bind them on your `app` in your own build. The examples on this page show both.

## Your first hook

Make `pb_hooks/posts.pb.ts` in your app folder:

```ts
// pb_hooks/posts.pb.ts
onRecordCreate((e) => {
  e.record.set("slug", slugify(e.record.getString("title")))
  e.next()
}, "posts")

function slugify(s: string): string {
  return s.toLowerCase().trim().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "")
}
```

```go
app.OnRecordCreate("posts").BindFunc(func(e *core.RecordEvent) error {
	e.Record.Set("slug", slugify(e.Record.GetString("title")))
	return e.Next()
})
```

Create a post (in the dashboard, or with `POST /api/collections/posts/records`) and it's saved with a slug. Three
things in that example hold for every hook:

- **The handler gets the event** (`e`): here the record being created, the app (`e.app`), and for request hooks the
  request and who made it.
- **`e.next()` lets the event go on**: to the next handler, then to the action itself. Code before `e.next()` runs
  before the action (the record isn't saved yet); code after it runs once the action is done.
- **Tags narrow a hook**: `"posts"` makes it run only for that collection. Pass several (`"posts", "pages"`), or none
  for every collection.

PocketOne runs `.pb.ts` files directly, with no build step, and writes the whole JS API's types to `types.d.ts` next to
`pb_hooks`, so your editor completes `e.record.` and flags mistakes. See [QuickJS hooks](/docs/quickjs) for imports,
async handlers, shared state and limits.

## Stop or change an action

Throw an error before `e.next()` and the action doesn't happen. In a hook triggered by an API request, the client gets
the error:

```ts
// pb_hooks/posts.pb.ts
onRecordCreateRequest((e) => {
  if (e.record.getString("title").length < 3) {
    throw new BadRequestError("A title needs at least 3 characters.")
  }
  e.next()
}, "posts")
```

```go
app.OnRecordCreateRequest("posts").BindFunc(func(e *core.RecordRequestEvent) error {
	if len(e.Record.GetString("title")) < 3 {
		return e.BadRequestError("A title needs at least 3 characters.", nil)
	}
	return e.Next()
})
```

The client gets `400` with `{"message": "A title needs at least 3 characters."}`. Change the event instead, and the
action goes on with your change: set a field on `e.record`, or filter what a list request returns.

## Before and after: which hook to use

Record hooks come in layers. Pick the one that matches when your code should run:

| You want to | Hook | Runs for |
|---|---|---|
| Fill in or check fields on every save, from anywhere (API, dashboard, your own code) | `onRecordCreate`, `onRecordUpdate`, `onRecordDelete`, `onRecordValidate` | every save |
| Do something once a save has really happened (send a mail, call a webhook) | `onRecordAfterCreateSuccess`, `onRecordAfterUpdateSuccess`, `onRecordAfterDeleteSuccess` | every save, after the transaction commits |
| Clean up when a save failed | `onRecordAfterCreateError` (and Update, Delete) | every failed save |
| Act on what a client asked for (check who's asking, change the input, change the answer) | `onRecordCreateRequest`, `onRecordUpdateRequest`, `onRecordDeleteRequest`, `onRecordViewRequest`, `onRecordsListRequest` | API requests only |
| Change how a record looks in every API answer (hide or add a field per user) | `onRecordEnrich` | every record the API returns, realtime included |

The model hooks (`onModelCreate` and the rest) are the same layers for anything saved, collections and logs included.
Collections have their own (`onCollectionCreate`...).

A side effect like sending mail belongs in an `AfterSuccess` hook: a plain `onRecordCreate` runs inside the save's
transaction, so if something later fails, the record is rolled back but your mail already went out.

```ts
// pb_hooks/welcome.pb.ts
onRecordAfterCreateSuccess((e) => {
  const message = new MailerMessage({
    from: { address: e.app.settings().meta.senderAddress, name: e.app.settings().meta.senderName },
    to: [{ address: e.record.email() }],
    subject: "Welcome!",
    html: "<p>Thanks for signing up.</p>",
  })
  e.app.newMailClient().send(message)
  e.next()
}, "users")
```

```go
app.OnRecordAfterCreateSuccess("users").BindFunc(func(e *core.RecordEvent) error {
	message := &mailer.Message{
		From:    mail.Address{Address: e.App.Settings().Meta.SenderAddress, Name: e.App.Settings().Meta.SenderName},
		To:      []mail.Address{{Address: e.Record.Email()}},
		Subject: "Welcome!",
		HTML:    "<p>Thanks for signing up.</p>",
	}
	if err := e.App.NewMailClient().Send(message); err != nil {
		e.App.Logger().Error("welcome mail", "error", err)
	}
	return e.Next()
})
```

## Who's asking

Request hooks know who made the request. `e.auth` is the signed-in record, or `null` for a guest; superusers are
records of the `_superusers` collection. Request hooks run after the collection's [API rule](/docs/api-rules) has let
the request through, so this hook decides only with a rule that lets guests in (an empty create rule); with a rule like
`@request.auth.id != ''`, the rule turns guests away first:

```ts
// pb_hooks/posts.pb.ts
onRecordCreateRequest((e) => {
  if (!e.auth) {
    throw new UnauthorizedError("Sign in to post.")
  }
  if (!e.hasSuperuserAuth()) {
    e.record.set("author", e.auth.id) // members can't post as someone else
  }
  e.next()
}, "posts")
```

```go
app.OnRecordCreateRequest("posts").BindFunc(func(e *core.RecordRequestEvent) error {
	if e.Auth == nil {
		return e.UnauthorizedError("Sign in to post.", nil)
	}
	if !e.HasSuperuserAuth() {
		e.Record.Set("author", e.Auth.Id)
	}
	return e.Next()
})
```

Most "who may do what" rules don't need a hook at all: [API rules](/docs/api-rules) express them as filters on the
collection. Use a hook when the rule needs code.

## The app's own events

Hooks with no record cover the app's life and its other parts:

| Hook | When |
|---|---|
| `onBootstrap` | the app starts and opens its database (after `e.next()` it's open) |
| `onServe` | the HTTP server starts (Go builds add routes here) |
| `onTerminate` | the app stops |
| `onMailerSend` | any email is about to go out (change it, or stop it) |
| `onRealtimeConnectRequest`, `onRealtimeSubscribeRequest`, `onRealtimeMessageSend` | realtime clients connect, subscribe and get messages |
| `onBackupCreate`, `onBackupRestore` | a backup is made or restored |
| `onSettingsReload` | the app's settings change |

Every hook, with its event's type, is in the [hooks table](/docs/reference/javascript/pocketbase#hooks) of the
JavaScript API, and on [`core.App`](/docs/reference/go/pocketbase/core#app) in Go.

## Order and errors

- Handlers run in the order they were bound: within a file from top to bottom, files in alphabetical order. In Go,
  `Bind(&hook.Handler{Id: "...", Priority: -10, Func: ...})` runs a handler earlier (lower first) and gives it an id
  you can `Unbind`.
- A handler that doesn't call `e.next()` stops the chain: later handlers and the action don't run. That's how you
  skip an action without an error. Most hooks should call it.
- An error thrown in a request hook becomes the response (`BadRequestError` is 400, `ForbiddenError` 403,
  `NotFoundError` 404, `UnauthorizedError` 401, `TooManyRequestsError` 429). Any other error is a 400 with a generic
  message, and its own text goes to the logs, so clients never see internals.
  An error in a model hook fails the save, and the request that caused it.

## When hooks reload

PocketOne watches `pb_hooks`. Change a file and the app's JavaScript restarts with the new code, in a moment, without
dropping the database or other requests. In development (`pocketone serve --dev`), a file with an error is skipped and
logged, so the rest keep working; in production the app stops on it, so a broken deploy shows up at once. A Go build
picks up hook changes when you rebuild and restart it.

## Next

- [Custom routes](/docs/routes): add your own API endpoints.
- [QuickJS hooks](/docs/quickjs): imports, `async` handlers, shared state, memory and time limits.
- [Records](/docs/records): read and write records from a hook.
- [JavaScript API](/docs/reference/javascript/pocketbase): every global, hook and type.
