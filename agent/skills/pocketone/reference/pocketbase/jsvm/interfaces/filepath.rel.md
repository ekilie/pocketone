Source: https://pocketbase.io/jsvm/interfaces/filepath.rel.html

# Interface rel

**Hierarchy**

- rel

```ts
rel(basePath, targPath): string
```

Rel returns a relative path that is lexically equivalent to targPath when joined to basePath with an intervening separator. That is, [Join](basePath, Rel(basePath, targPath)) is equivalent to targPath itself.

The returned path will always be relative to basePath, even if basePath and targPath share no elements. Rel calls [Clean] on the result.

An error is returned if targPath can't be made relative to basePath or if knowing the current working directory would be necessary to compute it.

**Parameters**

- `basePath: string`
- `targPath: string`

**Returns string**
