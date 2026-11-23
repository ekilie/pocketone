---
title: API rules and filters
description: Decide who can list, view, create, update and delete each collection's records with API rules, written in the same filter language clients use to search - with the request, auth and other collections in reach.
---

# API rules and filters

Every [collection](/docs/collections) has five API rules, one per action: **list**, **view**, **create**, **update**
and **delete**. A rule decides who may do that action through the [REST API](/docs/reference/rest), and it's written in
the same filter language clients use to search. Superusers skip the rules; your own code (`$app.save`, `findRecords...`)
isn't checked against them either.

## The three kinds of rule

| Rule | Means |
|---|---|
| unset (`null`, the default; a lock in the dashboard) | only superusers |
| empty string `""` | anyone, signed in or not |
| a filter, like `author = @request.auth.id` | whoever the filter matches |

Set them in the dashboard (the collection's **API rules** tab) or in a migration:

```ts
migrate((app) => {
  const posts = app.findCollectionByNameOrId("posts")
  posts.listRule = "published = true || author = @request.auth.id"
  posts.viewRule = "published = true || author = @request.auth.id"
  posts.createRule = "@request.auth.id != '' && @request.body.author = @request.auth.id"
  posts.updateRule = "author = @request.auth.id"
  posts.deleteRule = null // superusers only
  app.save(posts)
})
```

```go
m.Register(func(app core.App) error {
	posts, err := app.FindCollectionByNameOrId("posts")
	if err != nil {
		return err
	}
	posts.ListRule = types.Pointer("published = true || author = @request.auth.id")
	posts.ViewRule = types.Pointer("published = true || author = @request.auth.id")
	posts.CreateRule = types.Pointer("@request.auth.id != '' && @request.body.author = @request.auth.id")
	posts.UpdateRule = types.Pointer("author = @request.auth.id")
	posts.DeleteRule = nil
	return app.Save(posts)
}, nil)
```

## How a rule is applied

- **List** works as a filter: a client gets only the records the rule matches, never an error. A guest listing
  `posts` with the rule above sees only published posts.
- **View, update and delete** check the one record: if the rule doesn't match, the API answers 404, as if the record
  weren't there (so it doesn't leak which ids exist).
- **Create** checks the record that would be created, built from the request's body.
- Auth collections have a sixth rule, **manage**, for records allowed to change another record's email and password
  (a team admin, say).

## The filter language

```
title ~ 'hello' && (published = true || created > '2026-01-01 00:00:00')
```

| Operator | Means |
|---|---|
| `=` `!=` | equal, not equal |
| `>` `>=` `<` `<=` | greater, less |
| `~` `!~` | contains, doesn't contain (a `like`; `%` wildcards allowed) |
| `?=` `?!=` `?>` `?~` ... | the same, for any item of a multi-value field ("any tag is ...") |
| `&&` `\|\|` `( )` | and, or, grouping |

Values are quoted strings (`'text'`), numbers, `true`, `false` or `null`. Dates compare as `'YYYY-MM-DD HH:MM:SS'`
strings, in UTC. Fields of related records are reached with dots: `author.verified = true`.

## What a rule can see

| Identifier | Is |
|---|---|
| `@request.auth.*` | the signed-in record: `@request.auth.id` (empty for a guest), `@request.auth.collectionName`, any of its fields (`@request.auth.role = 'admin'`) |
| `@request.body.*` | the submitted values (create and update): `@request.body.status`, `@request.body.title:isset = true` |
| `@request.query.*` | the URL's query parameters |
| `@request.headers.*` | the request's headers, lowercase with `_` for `-` (`@request.headers.x_token`) |
| `@request.method`, `@request.context` | the HTTP method; where the request came from (`default`, `oauth2`, `otp`, `password`, `realtime`, `protectedFile`) |
| `@collection.<name>.*` | another collection's records, matched like a join: `@collection.members.user = @request.auth.id && @collection.members.team = team` |
| `@now`, `@today...`, `@yesterday`, `@tomorrow` ... | the current time and date parts (`@todayStart`, `@monthEnd`, `@year`) |

Modifiers on a field:

| Modifier | Gives |
|---|---|
| `:isset` | whether the client sent the field (`@request.body.role:isset = false`: may not set `role`) |
| `:changed` | whether an update changes it (`@request.body.owner:changed = false`) |
| `:length` | the number of items in a multi-value field (`tags:length <= 5`) |
| `:each` | applies to every item (`tags:each ~ 'blog-'`) |
| `:lower` | compares lowercase (`email:lower = 'ada@example.com'`) |

## Recipes

| Who | Rule |
|---|---|
| anyone | `""` |
| any signed-in user | `@request.auth.id != ''` |
| signed-in users of one collection | `@request.auth.collectionName = 'customers'` |
| only verified users | `@request.auth.verified = true` |
| the record's owner | `author = @request.auth.id` |
| published, or mine | `published = true \|\| author = @request.auth.id` |
| create only as yourself | `@request.auth.id != '' && @request.body.author = @request.auth.id` |
| update, but not the owner field | `author = @request.auth.id && @request.body.author:isset = false` |
| members of the record's team | `@collection.members.team = team && @collection.members.user = @request.auth.id` |
| users with a role | `@request.auth.role = 'editor'` |

## Filters in the API and in code

The same language filters lists through the REST API:

```bash
curl "http://127.0.0.1:8090/api/collections/posts/records?filter=(title~'hello')&sort=-created"
```

and in your code, where values go in as `{:params}` so they can't change the filter's meaning:

```ts
const mine = $app.findRecordsByFilter("posts", "author = {:me} && title ~ {:q}", "-created", 20, 0, {
  me: userId,
  q: search,
})
```

```go
mine, err := app.FindRecordsByFilter("posts", "author = {:me} && title ~ {:q}", "-created", 20, 0,
	dbx.Params{"me": userID, "q": search})
```

A rule is combined with the client's own `filter` (both must match), so clients can narrow what they may see, never
widen it.

## When a rule isn't enough

A rule answers "may this request do this to this record?" from data. When the answer needs code (a call to another
service, a limit per day, a check that spans many records), write a [hook](/docs/hooks) on the request
(`onRecordCreateRequest` and the rest) and throw a `ForbiddenError` there. Request hooks run after the rule: they see
only the requests it allows, so the two add up (the rule first, then your code).

## Next

- [Authentication](/docs/authentication): how a request gets `@request.auth`.
- [Records](/docs/records): filters in your own code.
- [Collections and fields](/docs/collections): where rules live.
