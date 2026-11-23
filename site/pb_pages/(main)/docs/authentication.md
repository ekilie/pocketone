---
title: Authentication
description: Sign users in with a password, a one-time code or an OAuth2 provider, add multi-factor auth, verify emails and reset passwords, and use the signed-in user in your rules and code.
---

# Authentication

Users sign in to an **auth collection**: a [collection](/docs/collections) whose records have an email, a password and
the sign-in methods you turn on. Every app has one, `users`, and `_superusers` for the dashboard; add more for
different kinds of account (`customers`, `staff`). Signing in returns a **token**; the client sends it with each
request, and the request then knows who it's from: `@request.auth` in [API rules](/docs/api-rules), `e.auth` in
[hooks](/docs/hooks) and [routes](/docs/routes).

## Sign-in methods

Turn them on per collection, in the dashboard (the collection's **Options**) or in code:

| Method | The user | Turn on |
|---|---|---|
| **Password** | enters an email (or another identity field) and a password | `passwordAuth` (on for `users`) |
| **One-time code** | asks for a code by email and enters it | `otp` |
| **OAuth2** | signs in with Google, GitHub, Apple, Microsoft and many more providers | `oauth2`, with each provider's client id and secret |
| **Multi-factor** | signs in with two methods (a password, then a code) | `mfa`, together with two of the above |

```ts
// pb_migrations/1732500000_users_signin.ts
migrate((app) => {
  const users = app.findCollectionByNameOrId("users")
  users.passwordAuth.enabled = true
  users.passwordAuth.identityFields = ["email"]
  users.otp.enabled = true
  users.otp.duration = 300 // a code lasts 5 minutes
  users.oauth2.enabled = true
  users.oauth2.providers = [{
    name: "google",
    clientId: $os.getenv("GOOGLE_CLIENT_ID"),
    clientSecret: $os.getenv("GOOGLE_CLIENT_SECRET"),
  }]
  app.save(users)
})
```

```go
m.Register(func(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	users.PasswordAuth.Enabled = true
	users.PasswordAuth.IdentityFields = []string{"email"}
	users.OTP.Enabled = true
	users.OTP.Duration = 300
	users.OAuth2.Enabled = true
	users.OAuth2.Providers = []core.OAuth2ProviderConfig{{
		Name:         "google",
		ClientId:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	}}
	return app.Save(users)
}, nil)
```

## Sign in from a client

With the JavaScript SDK (`npm install pocketbase`, it works with PocketOne unchanged):

```js
import PocketBase from "pocketbase"

const pb = new PocketBase("https://myapp.example.com")

// password
await pb.collection("users").authWithPassword("ada@example.com", "a long password")

// one-time code: ask, then enter the code from the email
const { otpId } = await pb.collection("users").requestOTP("ada@example.com")
await pb.collection("users").authWithOTP(otpId, "123456")

// OAuth2: opens the provider's sign-in in a popup
await pb.collection("users").authWithOAuth2({ provider: "google" })

pb.authStore.isValid // true
pb.authStore.record  // the signed-in user
pb.authStore.token   // sent with every request from now on
```

The same over HTTP:

```bash
curl -X POST http://127.0.0.1:8090/api/collections/users/auth-with-password \
  -H "Content-Type: application/json" \
  -d '{"identity": "ada@example.com", "password": "a long password"}'
# {"token": "eyJhbGciOi...", "record": {"id": "...", "email": "ada@example.com", ...}}

curl http://127.0.0.1:8090/api/collections/posts/records \
  -H "Authorization: eyJhbGciOi..."
```

Tokens expire (after 7 days for `users`, set per collection); `authRefresh` swaps a valid token for a fresh one. In
[Pages](/docs/extensions/pages), sign-in works through a cookie instead: see [Sign users in on your
pages](/docs/extensions/pages/authentication).

## Sign up, verify, reset

- **Sign up** is a create on the auth collection (`pb.collection("users").create({ email, password, passwordConfirm })`),
  allowed by its create rule (empty for `users`: anyone may sign up).
- **Verify an email:** `requestVerification(email)` sends a link; the user's `verified` field turns true when they open
  it. Require it in rules with `@request.auth.verified = true`.
- **Reset a password:** `requestPasswordReset(email)` sends a link to a page where the user picks a new one.
- **Change an email:** `requestEmailChange(newEmail)` sends a confirmation to the new address.

The emails' wording and links are set per collection (**Options** → mail templates), and they're sent through the
app's SMTP settings.

## Multi-factor auth

With `mfa` on, signing in with one method answers `401` with an `mfaId`; the client then signs in with a second method,
passing it:

```js
try {
  await pb.collection("users").authWithPassword(email, password)
} catch (err) {
  const mfaId = err.response?.mfaId
  if (!mfaId) throw err
  const { otpId } = await pb.collection("users").requestOTP(email)
  await pb.collection("users").authWithOTP(otpId, codeFromUser, { mfaId })
}
```

## The signed-in user in your code

In a hook or route, `e.auth` is the record that signed in, or `null`:

```ts
// pb_hooks/me.pb.ts
routerAdd("GET", "/api/me", (e) => {
  if (!e.auth) throw new UnauthorizedError("Sign in first.")
  return e.json(200, {
    id: e.auth.id,
    email: e.auth.email(),
    collection: e.auth.collection().name,
    superuser: e.hasSuperuserAuth(),
  })
})
```

```go
se.Router.GET("/api/me", func(e *core.RequestEvent) error {
	if e.Auth == nil {
		return e.UnauthorizedError("Sign in first.", nil)
	}
	return e.JSON(200, map[string]any{
		"id":         e.Auth.Id,
		"email":      e.Auth.Email(),
		"collection": e.Auth.Collection().Name,
		"superuser":  e.HasSuperuserAuth(),
	})
})
```

`$apis.requireAuth()` on a route does the check for you ([Custom routes](/docs/routes#require-sign-in)). To sign a user
in from your own route (a magic link, a code from another system), find the record and answer with a token:

```ts
// pb_hooks/magic.pb.ts
routerAdd("POST", "/api/magic/{code}", (e) => {
  const link = e.app.findFirstRecordByData("magic_links", "code", e.request.pathValue("code"))
  const user = e.app.findRecordById("users", link.getString("user"))
  e.app.delete(link) // one use
  return $apis.recordAuthResponse(e, user, "magic")
})
```

```go
se.Router.POST("/api/magic/{code}", func(e *core.RequestEvent) error {
	link, err := e.App.FindFirstRecordByData("magic_links", "code", e.Request.PathValue("code"))
	if err != nil {
		return e.NotFoundError("", err)
	}
	user, err := e.App.FindRecordById("users", link.GetString("user"))
	if err != nil {
		return err
	}
	if err := e.App.Delete(link); err != nil {
		return err
	}
	return apis.RecordAuthResponse(e, user, "magic", nil)
})
```

The answer is the same `{token, record}` as a password sign-in, so the SDK's auth store takes it as is.

## Impersonation

A superuser can get a token for any user, to see the app as they do (support cases). In the dashboard, open the
record and choose **Impersonate**; over the API, `POST /api/collections/users/impersonate/{id}` with a superuser's
token. The token can't be refreshed and lasts as long as you ask (`duration`, in seconds).

## Tokens for scripts and other apps

A user's sign-in token is for that user's session. For a script, a server or another app that calls yours, give it
an [API token](/docs/features/tokens): named, scoped to what it may do, and revoked one by one.

## Next

- [API rules and filters](/docs/api-rules): `@request.auth` in rules.
- [API tokens and OAuth](/docs/features/tokens): access for scripts, apps and agents.
- [Sign users in on your pages](/docs/extensions/pages/authentication): sessions in a Pages site.
- [Let users sign in with Google](/docs/extensions/pages/speedruns/google-oauth2): the provider setup, step by step.
