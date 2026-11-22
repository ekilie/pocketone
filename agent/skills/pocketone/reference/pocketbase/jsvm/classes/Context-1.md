Source: https://pocketbase.io/jsvm/classes/Context-1.html

# Class Context

Context creates a new empty Go context.Context.

This is usually used as part of some Go transitive bindings.

Example:

```js
const blank = new Context()

// with single key-value pair
const base = new Context(null, "a", 123)
console.log(base.value("a")) // 123

// extend with additional key-value pair
const sub = new Context(base, "b", 456)
console.log(sub.value("a")) // 123
console.log(sub.value("b")) // 456
```

**Hierarchy**

- [context](https://pocketbase.io/jsvm/modules/context.html).[Context](https://pocketbase.io/jsvm/interfaces/context.Context.html)

  - Context

**Implements**

- [context](https://pocketbase.io/jsvm/modules/context.html).[Context](https://pocketbase.io/jsvm/interfaces/context.Context.html)

## Constructors

### constructor

```ts
new Context(parentCtx?, key?, value?): Context
```

**Parameters**

- `Optional parentCtx: Context`
- `Optional key: any`
- `Optional value: any`

**Returns Context**

## Methods

### deadline

```ts
deadline(): [Time, boolean]
```

Deadline returns the time when work done on behalf of this context should be canceled. Deadline returns ok==false when no deadline is set. Successive calls to Deadline return the same results.

**Returns [Time, boolean]**

### done

```ts
done(): undefined
```

Done returns a channel that's closed when work done on behalf of this context should be canceled. Done may return nil if this context can never be canceled. Successive calls to Done return the same value. The close of the Done channel may happen asynchronously, after the cancel function returns.

WithCancel arranges for Done to be closed when cancel is called; WithDeadline arranges for Done to be closed when the deadline expires; WithTimeout arranges for Done to be closed when the timeout elapses.

Done is provided for use in select statements:

// Stream generates values with DoSomething and sends them to out // until DoSomething returns an error or ctx.Done is closed. func Stream(ctx context.Context, out chan<- Value) error { for { v, err := DoSomething(ctx) if err != nil { return err } select { case <-ctx.Done(): return ctx.Err() case out <- v: } } }

See [https://go.dev/blog/pipelines](https://go.dev/blog/pipelines) for more examples of how to use a Done channel for cancellation.

**Returns undefined**

### err

```ts
err(): void
```

If Done is not yet closed, Err returns nil. If Done is closed, Err returns a non-nil error explaining why: DeadlineExceeded if the context's deadline passed, or Canceled if the context was canceled for some other reason. After Err returns a non-nil error, successive calls to Err return the same error.

**Returns void**

### value

```ts
value(key): any
```

Value returns the value associated with this context for key, or nil if no value is associated with key. Successive calls to Value with the same key returns the same result.

Use context values only for request-scoped data that transits processes and API boundaries, not for passing optional parameters to functions.

A key identifies a specific value in a Context. Functions that wish to store values in Context typically allocate a key in a global variable then use that key as the argument to context.WithValue and Context.Value. A key can be any type that supports equality; packages should define keys as an unexported type to avoid collisions.

Packages that define a Context key should provide type-safe accessors for the values stored using that key:

```
 // Package user defines a User type that's stored in Contexts.
 package user

 import "context"

 // User is the type of value stored in the Contexts.
 type User struct {...}

 // key is an unexported type for keys defined in this package.
 // This prevents collisions with keys defined in other packages.
 type key int

 // userKey is the key for user.User values in Contexts. It is
 // unexported; clients use user.NewContext and user.FromContext
 // instead of using this key directly.
 var userKey key

 // NewContext returns a new Context that carries value u.
 func NewContext(ctx context.Context, u *User) context.Context {
   return context.WithValue(ctx, userKey, u)
 }

 // FromContext returns the User value stored in ctx, if any.
 func FromContext(ctx context.Context) (*User, bool) {
   u, ok := ctx.Value(userKey).(*User)
   return u, ok
 }
```

**Parameters**

- `key: any`

**Returns any**
