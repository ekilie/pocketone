Source: https://pocketbase.io/jsvm/interfaces/core.RecordProxy.html

# Interface RecordProxy

RecordProxy defines an interface for a Record proxy/project model, aka. custom model struct that acts on behalve the proxied Record to allow for example typed getter/setters for the Record fields.

To implement the interface it is usually enough to embed the [BaseRecordProxy] struct.

**Hierarchy**

- RecordProxy

**Indexable**

```ts
[key: string]: any
```

## Methods

### proxyRecord

```ts
proxyRecord(): core.Record
```

ProxyRecord returns the proxied Record model.

**Returns core.Record**

### setProxyRecord

```ts
setProxyRecord(record): void
```

SetProxyRecord loads the specified record model into the current proxy.

**Parameters**

- `record: core.Record`

**Returns void**
