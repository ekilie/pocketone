Source: https://pocketbase.io/jsvm/interfaces/core.newBaseCollection.html

# Interface newBaseCollection

**Hierarchy**

- newBaseCollection

```ts
newBaseCollection(name, ...optId): core.Collection
```

NewBaseCollection initializes and returns a new "base" Collection model.

It also loads the minimal default configuration for the collection (eg. system fields, indexes, type specific options, etc.).

**Parameters**

- `name: string`
- `Rest ...optId: string[]`

**Returns core.Collection**
