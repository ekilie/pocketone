Source: https://pocketbase.io/jsvm/interfaces/apis.maxBytesReader.html

# Interface maxBytesReader

maxBytesReader is very similar to the http.MaxBytesReader but support rereads and doesn't try to prematurely close the related response to allow consequent middlewares to operate correctly.

**Hierarchy**

- [_ssZOHux](https://pocketbase.io/jsvm/types/apis._ssZOHux.html)

  - maxBytesReader

## Methods

### close

```ts
close(): void
```

**Returns void**

### read

```ts
read(b): number
```

**Parameters**

- `b: string | number[]`

**Returns number**

### reread

```ts
reread(): void
```

**Returns void**
