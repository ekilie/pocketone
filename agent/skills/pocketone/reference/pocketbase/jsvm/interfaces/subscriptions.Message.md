Source: https://pocketbase.io/jsvm/interfaces/subscriptions.Message.html

# Interface Message

Message defines a client's channel data.

**Hierarchy**

- Message

  - [SubscriptionMessage](https://pocketbase.io/jsvm/classes/SubscriptionMessage.html)

**Implemented by**

- [SubscriptionMessage](https://pocketbase.io/jsvm/classes/SubscriptionMessage.html)

## Methods

### writeSSE

```ts
writeSSE(w, eventId): void
```

WriteSSE writes the current message in a SSE format into the provided writer.

For example, writing to a router.Event:

```
 m := Message{Name: "users/create", Data: []byte{...}}
 m.WriteSSE(e.Response, "yourEventId")
 e.Flush()
```

**Parameters**

- `w: io.Writer`
- `eventId: string`

**Returns void**

## Properties

### data

```ts
data: string | number[]
```

### name

```ts
name: string
```
