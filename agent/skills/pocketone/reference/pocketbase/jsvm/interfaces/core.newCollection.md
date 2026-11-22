Source: https://pocketbase.io/jsvm/interfaces/core.newCollection.html

# Interface newCollection

**Hierarchy**

- newCollection

```ts
newCollection(typ, name, ...optId): core.Collection
```

NewCollection initializes and returns a new Collection model with the specified type and name.

It also loads the minimal default configuration for the collection (eg. system fields, indexes, type specific options, etc.).

**Parameters**

- `typ: string`
- `name: string`
- `Rest ...optId: string[]`

**Returns core.Collection**
