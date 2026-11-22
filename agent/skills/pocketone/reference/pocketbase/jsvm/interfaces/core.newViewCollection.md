Source: https://pocketbase.io/jsvm/interfaces/core.newViewCollection.html

# Interface newViewCollection

**Hierarchy**

- newViewCollection

```ts
newViewCollection(name, ...optId): core.Collection
```

NewViewCollection initializes and returns a new "view" Collection model.

It also loads the minimal default configuration for the collection (eg. system fields, indexes, type specific options, etc.).

**Parameters**

- `name: string`
- `Rest ...optId: string[]`

**Returns core.Collection**
