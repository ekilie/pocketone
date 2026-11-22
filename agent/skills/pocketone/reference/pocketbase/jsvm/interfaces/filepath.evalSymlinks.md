Source: https://pocketbase.io/jsvm/interfaces/filepath.evalSymlinks.html

# Interface evalSymlinks

**Hierarchy**

- evalSymlinks

```ts
evalSymlinks(path): string
```

EvalSymlinks returns the path name after the evaluation of any symbolic links. If path is relative the result will be relative to the current directory, unless one of the components is an absolute symbolic link. EvalSymlinks calls [Clean] on the result.

**Parameters**

- `path: string`

**Returns string**
