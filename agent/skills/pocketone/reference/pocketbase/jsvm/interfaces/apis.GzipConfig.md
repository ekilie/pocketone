Source: https://pocketbase.io/jsvm/interfaces/apis.GzipConfig.html

# Interface GzipConfig

GzipConfig defines the config for Gzip middleware.

**Hierarchy**

- GzipConfig

## Properties

### level

```ts
level: number
```

Gzip compression level. Optional. Default value -1.

### minLength

```ts
minLength: number
```

Length threshold before gzip compression is applied. Optional. Default value 0.

Most of the time you will not need to change the default. Compressing a short response might increase the transmitted data because of the gzip format overhead. Compressing the response will also consume CPU and time on the server and the client (for decompressing). Depending on your use case such a threshold might be useful.

See also: [https://webmasters.stackexchange.com/questions/31750/what-is-recommended-minimum-object-size-for-gzip-performance-benefits](https://webmasters.stackexchange.com/questions/31750/what-is-recommended-minimum-object-size-for-gzip-performance-benefits)
