---
title: Authentication API
description: Sign in to an auth collection over HTTP with a password, a one-time code or OAuth2, refresh tokens, verify emails, reset passwords, change emails and impersonate users.
---

# Authentication API

Auth endpoints are per [auth collection](/docs/authentication), at `/api/collections/{collection}/...`. A sign-in
answers a token and the record; send the token as the `Authorization` header from then on.

```json
{
  "token": "eyJhbGciOiJIUzI1NiJ9...",
  "record": { "collectionName": "users", "id": "0slkeutr0bxpw3v", "email": "ada@example.com", "verified": true }
}
```

That's the answer of every sign-in below. With multi-factor auth on, the first sign-in answers `401` with
`{"mfaId": "..."}` instead; pass that `mfaId` to the second sign-in.

## Sign-in methods

`GET /api/collections/{collection}/auth-methods`

Which methods the collection allows, for building a sign-in form. Anyone may call it.

```json
{
  "password": { "enabled": true, "identityFields": ["email"] },
  "oauth2": { "enabled": true, "providers": [{ "name": "google", "displayName": "Google", "state": "...", "authURL": "https://accounts.google.com/...", "codeVerifier": "...", "codeChallenge": "...", "codeChallengeMethod": "S256" }] },
  "mfa": { "enabled": false, "duration": 0 },
  "otp": { "enabled": true, "duration": 300 }
}
```

## Password

`POST /api/collections/{collection}/auth-with-password`

| Body | |
|---|---|
| `identity` | the email (or another identity field's value) |
| `password` | the password |
| `identityField` | optional: which identity field `identity` is |

```bash
curl -X POST http://127.0.0.1:8090/api/collections/users/auth-with-password \
  -H "Content-Type: application/json" \
  -d '{"identity": "ada@example.com", "password": "a long password"}'
```

## One-time code

`POST /api/collections/{collection}/request-otp` with `{"email": "..."}` emails a code and answers `{"otpId": "..."}`
(also for an email with no account, so it doesn't reveal which exist).

`POST /api/collections/{collection}/auth-with-otp` with `{"otpId": "...", "password": "<the code>"}` signs in.

```bash
curl -X POST http://127.0.0.1:8090/api/collections/users/request-otp \
  -H "Content-Type: application/json" -d '{"email": "ada@example.com"}'
# {"otpId": "8xk1r4t2y0p9q3z"}

curl -X POST http://127.0.0.1:8090/api/collections/users/auth-with-otp \
  -H "Content-Type: application/json" -d '{"otpId": "8xk1r4t2y0p9q3z", "password": "47201956"}'
```

## OAuth2

`POST /api/collections/{collection}/auth-with-oauth2`

After the user signs in at the provider (`authURL` from `auth-methods`) and comes back to your redirect URL with a
`code`, exchange it:

| Body | |
|---|---|
| `provider` | the provider's name (`google`) |
| `code` | the code the provider sent back |
| `codeVerifier` | the `codeVerifier` from `auth-methods` |
| `redirectURL` | the redirect URL you sent the user to the provider with |
| `createData` | optional: fields for the record, when this sign-in creates it |

The answer also has `meta`: the provider's profile (name, avatar, email). A first sign-in creates the record, if the
collection's create rule allows it.

The JS SDK's `authWithOAuth2` does all of this in a popup, through a redirect URL every app serves for that purpose:
`GET /api/oauth2-redirect` (and `POST /api/oauth2-redirect`, for providers that post back). Register
`https://<your app>/api/oauth2-redirect` with the provider.

## Refresh

`POST /api/collections/{collection}/auth-refresh`

With a valid token, answers a fresh one (and the current record). Use it to keep a session going, and to check that a
token is still good.

## Verify an email

`POST /api/collections/{collection}/request-verification` with `{"email": "..."}` emails a link (answers `204`).

`POST /api/collections/{collection}/confirm-verification` with `{"token": "..."}`, the token from the link, marks the
record `verified` (answers `204`).

## Reset a password

`POST /api/collections/{collection}/request-password-reset` with `{"email": "..."}` emails a link (answers `204`).

`POST /api/collections/{collection}/confirm-password-reset` with `{"token": "...", "password": "...",
"passwordConfirm": "..."}` sets the new password (answers `204`) and signs the record out of its other sessions.

## Change an email

`POST /api/collections/{collection}/request-email-change` with `{"newEmail": "..."}`, signed in, emails a link to the
new address (answers `204`).

`POST /api/collections/{collection}/confirm-email-change` with `{"token": "...", "password": "..."}` changes it
(answers `204`).

## Impersonate

`POST /api/collections/{collection}/impersonate/{id}` (**superusers**)

| Body | |
|---|---|
| `duration` | optional: how long the token lasts, in seconds (default: the collection's auth token duration) |

Answers a sign-in for that record: a token that can't be refreshed. For support: see the app as a user does.

## Next

- [Authentication](/docs/authentication): sign-in methods, settings and the signed-in user in your code.
- [Records API](/docs/rest/records): create a record in an auth collection to sign up.
