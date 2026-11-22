Source: https://pocketbase.io/jsvm/classes/SubscriptionMessage.html

# Class SubscriptionMessage

SubscriptionMessage defines a realtime subscription payload.

Example:

```js
onRealtimeConnectRequest((e) => {
    e.client.send(new SubscriptionMessage({
        name: "example",
        data: '{"greeting": "Hello world"}'
    }))
})
```

**Hierarchy**

- [subscriptions](https://pocketbase.io/jsvm/modules/subscriptions.html).[Message](https://pocketbase.io/jsvm/interfaces/subscriptions.Message.html)

  - SubscriptionMessage

**Implements**

- [subscriptions](https://pocketbase.io/jsvm/modules/subscriptions.html).[Message](https://pocketbase.io/jsvm/interfaces/subscriptions.Message.html)

## Constructors

### constructor

```ts
new SubscriptionMessage(options?): SubscriptionMessage
```

**Parameters**

- `Optional options: Partial<subscriptions.Message>`

**Returns SubscriptionMessage**

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
