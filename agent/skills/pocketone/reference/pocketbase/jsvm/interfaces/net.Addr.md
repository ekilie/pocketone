Source: https://pocketbase.io/jsvm/interfaces/net.Addr.html

# Interface Addr

Addr represents a network end point address.

The two methods [Addr.Network] and [Addr.String] conventionally return strings that can be passed as the arguments to [Dial], but the exact form and meaning of the strings is up to the implementation.

**Hierarchy**

- Addr

**Indexable**

```ts
[key: string]: any
```

## Methods

### network

```ts
network(): string
```

**Returns string**

### string

```ts
string(): string
```

**Returns string**
