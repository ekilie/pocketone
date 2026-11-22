Source: https://pocketbase.io/jsvm/interfaces/apis.bodyLimit.html

# Interface bodyLimit

**Hierarchy**

- bodyLimit

```ts
bodyLimit(limitBytes): hook.Handler<RequestEvent>
```

BodyLimit returns a middleware handler that changes the default request body size limit.

If limitBytes <= 0, no limit is applied.

Otherwise, if the request body size exceeds the configured limitBytes, it sends 413 error response.

**Parameters**

- `limitBytes: number`

**Returns hook.Handler<RequestEvent>**
