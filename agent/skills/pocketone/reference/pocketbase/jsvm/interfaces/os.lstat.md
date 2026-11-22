Source: https://pocketbase.io/jsvm/interfaces/os.lstat.html

# Interface lstat

**Hierarchy**

- lstat

```ts
lstat(name): os.FileInfo
```

Lstat returns a [FileInfo] describing the named file. If the file is a symbolic link, the returned FileInfo describes the symbolic link. Lstat makes no attempt to follow the link. If there is an error, it will be of type [*PathError].

On Windows, if the file is a reparse point that is a surrogate for another named entity (such as a symbolic link or mounted folder), the returned FileInfo describes the reparse point, and makes no attempt to resolve it.

**Parameters**

- `name: string`

**Returns os.FileInfo**
