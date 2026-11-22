Source: https://pocketbase.io/jsvm/interfaces/subscriptions.Broker.html

# Interface Broker

Broker defines a struct for managing subscriptions clients.

**Hierarchy**

- Broker

## Methods

### chunkedClients

```ts
chunkedClients(chunkSize): Client[][]
```

ChunkedClients splits the current clients into a chunked slice.

**Parameters**

- `chunkSize: number`

**Returns Client[][]**

### clientById

```ts
clientById(clientId): Client
```

ClientById finds a registered client by its id.

Returns non-nil error when client with clientId is not registered.

**Parameters**

- `clientId: string`

**Returns Client**

### clients

```ts
clients(): _TygojaDict
```

Clients returns a shallow copy of all registered clients indexed with their connection id.

**Returns _TygojaDict**

### register

```ts
register(client): void
```

Register adds a new client to the broker instance.

**Parameters**

- `client: Client`

**Returns void**

### totalClients

```ts
totalClients(): number
```

TotalClients returns the total number of registered clients.

**Returns number**

### unregister

```ts
unregister(clientId): void
```

Unregister removes a single client by its id and marks it as discarded.

If client with clientId doesn't exist, this method does nothing.

**Parameters**

- `clientId: string`

**Returns void**
