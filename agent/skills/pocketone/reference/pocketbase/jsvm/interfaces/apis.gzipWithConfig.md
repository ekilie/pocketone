Source: https://pocketbase.io/jsvm/interfaces/apis.gzipWithConfig.html

# Interface gzipWithConfig

**Hierarchy**

- gzipWithConfig

```ts
gzipWithConfig(config): hook.Handler<RequestEvent>
```

GzipWithConfig returns a middleware which compresses HTTP response using gzip compression scheme.

**Parameters**

- `config: GzipConfig`

**Returns hook.Handler<RequestEvent>**
