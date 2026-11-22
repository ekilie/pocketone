Source: https://pocketbase.io/jsvm/interfaces/net.Listener.html

# Interface Listener

A Listener is a generic network listener for stream-oriented protocols.

Multiple goroutines may invoke methods on a Listener simultaneously.

**Hierarchy**

- Listener

**Indexable**

```ts
[key: string]: any
```

## Methods

### accept

```ts
accept(): net.Conn
```

Accept waits for and returns the next connection to the listener.

**Returns net.Conn**

### addr

```ts
addr(): Addr
```

Addr returns the listener's network address.

**Returns Addr**

### close

```ts
close(): void
```

Close closes the listener. Any blocked Accept operations will be unblocked and return errors.

**Returns void**
