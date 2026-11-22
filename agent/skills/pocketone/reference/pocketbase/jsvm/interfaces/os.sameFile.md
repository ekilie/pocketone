Source: https://pocketbase.io/jsvm/interfaces/os.sameFile.html

# Interface sameFile

**Hierarchy**

- sameFile

```ts
sameFile(fi1, fi2): boolean
```

SameFile reports whether fi1 and fi2 describe the same file. For example, on Unix this means that the device and inode fields of the two underlying structures are identical; on other systems the decision may be based on the path names. SameFile only applies to results returned by this package's [Stat]. It returns false in other cases.

**Parameters**

- `fi1: os.FileInfo`
- `fi2: os.FileInfo`

**Returns boolean**
