Source: https://pocketbase.io/jsvm/interfaces/http.PushOptions.html

# Interface PushOptions

PushOptions describes options for [Pusher.Push].

**Hierarchy**

- PushOptions

## Properties

### header

```ts
header: Header
```

Header specifies additional promised request headers. This cannot include HTTP/2 pseudo header fields like ":path" and ":scheme", which will be added automatically.

### method

```ts
method: string
```

Method specifies the HTTP method for the promised request. If set, it must be "GET" or "HEAD". Empty means "GET".
