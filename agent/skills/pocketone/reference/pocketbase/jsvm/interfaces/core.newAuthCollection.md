Source: https://pocketbase.io/jsvm/interfaces/core.newAuthCollection.html

# Interface newAuthCollection

**Hierarchy**

- newAuthCollection

```ts
newAuthCollection(name, ...optId): core.Collection
```

NewAuthCollection initializes and returns a new "auth" Collection model.

It also loads the minimal default configuration for the collection (eg. system fields, indexes, type specific options, etc.).

**Parameters**

- `name: string`
- `Rest ...optId: string[]`

**Returns core.Collection**
