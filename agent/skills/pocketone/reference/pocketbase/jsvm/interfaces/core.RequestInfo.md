Source: https://pocketbase.io/jsvm/interfaces/core.RequestInfo.html

# Interface RequestInfo

RequestInfo defines a HTTP request data struct, usually used as part of the `@request.*` filter resolver.

The Query and Headers fields contains only the first value for each found entry.

**Hierarchy**

- RequestInfo

## Methods

### clone

```ts
clone(): RequestInfo
```

Clone creates a new shallow copy of the current RequestInfo and its Auth record (if any).

**Returns RequestInfo**

### hasSuperuserAuth

```ts
hasSuperuserAuth(): boolean
```

HasSuperuserAuth checks whether the current RequestInfo instance has superuser authentication loaded.

**Returns boolean**

## Properties

### Optional auth

```ts
auth?: core.Record
```

### body

```ts
body: _TygojaDict
```

### context

```ts
context: string
```

### headers

```ts
headers: _TygojaDict
```

### method

```ts
method: string
```

### query

```ts
query: _TygojaDict
```
