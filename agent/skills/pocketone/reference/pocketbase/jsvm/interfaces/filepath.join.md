Source: https://pocketbase.io/jsvm/interfaces/filepath.join.html

# Interface join

**Hierarchy**

- join

```ts
join(...elem): string
```

Join joins any number of path elements into a single path, separating them with an OS specific [Separator]. Empty elements are ignored. The result is Cleaned. However, if the argument list is empty or all its elements are empty, Join returns an empty string. On Windows, the result will only be a UNC path if the first non-empty element is a UNC path.

**Parameters**

- `Rest ...elem: string[]`

**Returns string**
