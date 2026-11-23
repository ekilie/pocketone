---
title: Records
description: Find, create, change and delete records from your own code - filters with parameters, sorting and paging, reading typed values, expanding relations, transactions and raw SQL.
---

# Records

A record is one row of a [collection](/docs/collections). Clients reach records through the [REST
API](/docs/reference/rest); this page is about your own code: a [hook](/docs/hooks), a [route](/docs/routes), a cron
job or a migration. In JavaScript the app is `$app` (or `e.app` inside a handler, which is the right one inside a
transaction); in Go it's your `app`.

## Find records

```ts
// one by id (throws a not-found error when it doesn't exist)
const post = $app.findRecordById("posts", "abc123def456ghi")

// many, by filter: collection, filter, sort, limit, offset, params
const recent = $app.findRecordsByFilter(
  "posts",
  "published = true && created > {:since}",
  "-created",
  20,
  0,
  { since: "2026-01-01 00:00:00" },
)

// the first match, or an error when there's none
const bySlug = $app.findFirstRecordByFilter("posts", "slug = {:slug}", { slug: "hello-world" })

// by one field's exact value
const sameSlug = $app.findFirstRecordByData("posts", "slug", "hello-world")

// how many
const drafts = $app.countRecords("posts", $dbx.hashExp({ published: false }))
```

```go
post, err := app.FindRecordById("posts", "abc123def456ghi")

recent, err := app.FindRecordsByFilter(
	"posts",
	"published = true && created > {:since}",
	"-created",
	20,
	0,
	dbx.Params{"since": "2026-01-01 00:00:00"},
)

bySlug, err := app.FindFirstRecordByFilter("posts", "slug = {:slug}", dbx.Params{"slug": "hello-world"})

sameSlug, err := app.FindFirstRecordByData("posts", "slug", "hello-world")

drafts, err := app.CountRecords("posts", dbx.HashExp{"published": false})
```

The filter is the same language as [API rules](/docs/api-rules) and the API's `?filter=`. **Always pass values as
`{:name}` parameters**, never by building the string, so a value can't change the filter's meaning. The sort is a
comma list of fields, `-` for descending (`"-created,title"`). A limit of `0` means no limit.

Not finding a record is an error (`findRecordById`, `findFirstRecordBy...`), which a route turns into a 404. To check
without failing, catch it, or use `findRecordsByFilter` and look at the length.

## Read values

`get` returns a field's value as it is; the typed getters convert:

```ts
const title = post.getString("title")
const views = post.getInt("views")
const published = post.getBool("published")
const created = post.getDateTime("created") // a DateTime: .time(), .string(), .isZero()
const tags = post.getStringSlice("tags")   // multi-select, multi-relation, multi-file
const meta = post.get("meta")               // json: the parsed value
const id = post.id
```

```go
title := post.GetString("title")
views := post.GetInt("views")
published := post.GetBool("published")
created := post.GetDateTime("created")
tags := post.GetStringSlice("tags")
meta := post.Get("meta")
id := post.Id
```

An auth record also has `email()`, `verified()` and `validatePassword(password)` (in Go `Email()`, `Verified()`,
`ValidatePassword`).

## Create, change and delete

```ts
const posts = $app.findCollectionByNameOrId("posts")
const post = new Record(posts)
post.set("title", "Hello")
post.set("author", userId)
$app.save(post) // validates, runs hooks, then inserts; post.id is set

post.set("title", "Hello again")
$app.save(post) // updates

$app.delete(post)
```

```go
posts, err := app.FindCollectionByNameOrId("posts")
if err != nil {
	return err
}
post := core.NewRecord(posts)
post.Set("title", "Hello")
post.Set("author", userID)
if err := app.Save(post); err != nil {
	return err
}

post.Set("title", "Hello again")
if err := app.Save(post); err != nil {
	return err
}

if err := app.Delete(post); err != nil {
	return err
}
```

- **`save` validates first** (required fields, lengths, patterns, unique indexes) and fails with each field's problem.
  `saveNoValidate` skips that; use it only for data you've already checked.
- **Hooks run on every save,** whether the save came from the API, the dashboard or your code. Use
  `$app.unsafeWithoutHooks().save(record)` (Go `app.UnsafeWithoutHooks().Save`) to skip them, for example inside a hook
  that would otherwise trigger itself.
- **Set a field by its modifier**, as the API does: `post.set("views+", 1)` adds one, `post.set("tags+", "new")`
  appends, `post.set("tags-", "old")` removes.
- **Files:** set a file field to a file made with `$filesystem.fileFromPath`, `fileFromBytes` or `fileFromURL`
  (Go `filesystem.NewFileFrom...`); saving uploads it to the app's storage
  ([`$filesystem`](/docs/reference/javascript/pocketbase#filesystem)).

## Relations

A relation field holds ids. To get the related records, expand it:

```ts
const post = $app.findRecordById("posts", id)
$app.expandRecord(post, ["author"], null)
const author = post.expandedOne("author") // the users record, or null
const authorName = author?.getString("name")

// or follow the relation by hand
const sameAuthor = $app.findRecordById("users", post.getString("author"))
```

```go
post, err := app.FindRecordById("posts", id)
if err != nil {
	return err
}
if errs := app.ExpandRecord(post, []string{"author"}, nil); len(errs) > 0 {
	return fmt.Errorf("expand: %v", errs)
}
author := post.ExpandedOne("author")
```

`expandedAll(field)` returns the list for a multi-relation. To find the records that point *to* a record (a user's
posts), filter on the relation: `findRecordsByFilter("posts", "author = {:id}", ...)`. In the REST API, the same is
`?expand=author` and, backwards, `?expand=posts_via_author`.

## Transactions

Several saves that must all happen or none:

```ts
$app.runInTransaction((txApp) => {
  const order = new Record(txApp.findCollectionByNameOrId("orders"))
  order.set("total", 42)
  txApp.save(order)

  const stock = txApp.findRecordById("products", productId)
  stock.set("stock-", 1)
  txApp.save(stock) // throw (or fail) here and the order isn't saved either
})
```

```go
err := app.RunInTransaction(func(txApp core.App) error {
	orders, err := txApp.FindCollectionByNameOrId("orders")
	if err != nil {
		return err
	}
	order := core.NewRecord(orders)
	order.Set("total", 42)
	if err := txApp.Save(order); err != nil {
		return err
	}
	stock, err := txApp.FindRecordById("products", productID)
	if err != nil {
		return err
	}
	stock.Set("stock-", 1)
	return txApp.Save(stock)
})
```

Inside the function, use `txApp` for everything; `$app` would wait for the transaction's own lock. Keep it short:
SQLite allows one writer at a time, so a long transaction holds up every other save.

## Queries and raw SQL

For what a filter can't say (joins, aggregates, `GROUP BY`), write the query. The query builder:

```ts
const rows = arrayOf(new DynamicModel({ author: "", posts: 0 }))
$app.db()
  .select("author", "COUNT(*) AS posts")
  .from("posts")
  .where($dbx.exp("published = {:p}", { p: true }))
  .groupBy("author")
  .orderBy("posts DESC")
  .limit(10)
  .all(rows)
```

```go
rows := []struct {
	Author string `db:"author" json:"author"`
	Posts  int    `db:"posts" json:"posts"`
}{}
err := app.DB().
	Select("author", "COUNT(*) AS posts").
	From("posts").
	Where(dbx.NewExp("published = {:p}", dbx.Params{"p": true})).
	GroupBy("author").
	OrderBy("posts DESC").
	Limit(10).
	All(&rows)
```

Or plain SQL, with bound parameters:

```ts
const result = new DynamicModel({ total: 0 })
$app.db()
  .newQuery("SELECT COUNT(*) AS total FROM posts WHERE author = {:author}")
  .bind({ author: userId })
  .one(result)
```

```go
var result struct {
	Total int `db:"total"`
}
err := app.DB().
	NewQuery("SELECT COUNT(*) AS total FROM posts WHERE author = {:author}").
	Bind(dbx.Params{"author": userID}).
	One(&result)
```

To get records (not plain rows) from a query, start from `$app.recordQuery("posts")` and finish with `.all(records)`
on `arrayOf(new Record())`. `$dbx` has the expression helpers: `hashExp`, `exp`, `and`, `or`, `not`, `in`, `like`,
`between` and the rest.

## Next

- [API rules and filters](/docs/api-rules): the filter language in full.
- [Hooks](/docs/hooks): run code when records change.
- [`core.Record`](/docs/reference/javascript/pocketbase/core/records#corerecord) and
  [`core.App`](/docs/reference/javascript/pocketbase/core#coreapp) in the JavaScript API; in Go,
  [`core.Record`](/docs/reference/go/pocketbase/core/records#record) and
  [`core.App`](/docs/reference/go/pocketbase/core#app).
