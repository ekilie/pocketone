Source: https://pocketbase.io/jsvm/interfaces/os.stat.html

# Interface stat

**Hierarchy**

- stat

```ts
stat(name): os.FileInfo
```

Stat returns a [FileInfo] describing the named file. If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`

**Returns os.FileInfo**
