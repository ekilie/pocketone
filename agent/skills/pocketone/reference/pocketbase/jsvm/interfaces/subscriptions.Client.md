Source: https://pocketbase.io/jsvm/interfaces/subscriptions.Client.html

# Interface Client

Client is an interface for a generic subscription client.

**Hierarchy**

- Client

**Indexable**

```ts
[key: string]: any
```

## Methods

### channel

```ts
channel(): undefined
```

Channel returns the client's communication channel.

NB! The channel shouldn't be used after calling Discard().

**Returns undefined**

### discard

```ts
discard(): void
```

Discard marks the client as "discarded" (and closes its channel), meaning that it shouldn't be used anymore for sending new messages.

It is safe to call Discard() multiple times.

**Returns void**

### get

```ts
get(key): any
```

Get retrieves the key value from the client's context.

**Parameters**

- `key: string`

**Returns any**

### hasSubscription

```ts
hasSubscription(sub): boolean
```

HasSubscription checks if the client is subscribed to `sub`.

**Parameters**

- `sub: string`

**Returns boolean**

### id

```ts
id(): string
```

Id Returns the unique id of the client.

**Returns string**

### isDiscarded

```ts
isDiscarded(): boolean
```

IsDiscarded indicates whether the client has been "discarded" and should no longer be used.

**Returns boolean**

### send

```ts
send(m): void
```

Send sends the specified message to the client's channel (if not discarded).

**Parameters**

- `m: subscriptions.Message`

**Returns void**

### set

```ts
set(key, value): void
```

Set stores any value to the client's context.

**Parameters**

- `key: string`
- `value: any`

**Returns void**

### subscribe

```ts
subscribe(...subs): void
```

Subscribe subscribes the client to the provided subscriptions list.

Each subscription can also have "options" (json serialized SubscriptionOptions) as query parameter.

Example:

```
 Subscribe(
     "subscriptionA",
     `subscriptionB?options={"query":{"a":1},"headers":{"x_token":"abc"}}`,
 )
```

**Parameters**

- `Rest ...subs: string[]`

**Returns void**

### subscriptions

```ts
subscriptions(...prefixes): _TygojaDict
```

Subscriptions returns a shallow copy of the client subscriptions matching the prefixes. If no prefix is specified, returns all subscriptions.

**Parameters**

- `Rest ...prefixes: string[]`

**Returns _TygojaDict**

### unset

```ts
unset(key): void
```

Unset removes a single value from the client's context.

**Parameters**

- `key: string`

**Returns void**

### unsubscribe

```ts
unsubscribe(...subs): void
```

Unsubscribe unsubscribes the client from the provided subscriptions list.

**Parameters**

- `Rest ...subs: string[]`

**Returns void**
