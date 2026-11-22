Source: https://pocketbase.io/jsvm/interfaces/apis.requireSameCollectionContextAuth.html

# Interface requireSameCollectionContextAuth

**Hierarchy**

- requireSameCollectionContextAuth

```ts
requireSameCollectionContextAuth(collectionPathParam): hook.Handler<RequestEvent>
```

RequireSameCollectionContextAuth middleware requires a request to have a valid record Authorization header and the auth record's collection to match the one from the route path parameter (default to "collection" if collectionParam is empty).

**Parameters**

- `collectionPathParam: string`

**Returns hook.Handler<RequestEvent>**
