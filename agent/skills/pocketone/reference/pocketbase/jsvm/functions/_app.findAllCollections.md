Source: https://pocketbase.io/jsvm/functions/_app.findAllCollections.html

# Function findAllCollections

```ts
findAllCollections(...collectionTypes): core.Collection[]
```

FindCollections finds all collections by the given type(s).

If collectionTypes is not set, it returns all collections.

Example:

```
 app.FindAllCollections() // all collections
 app.FindAllCollections("auth", "view") // only auth and view collections
```

**Parameters**

- `Rest ...collectionTypes: string[]`

**Returns core.Collection[]**
