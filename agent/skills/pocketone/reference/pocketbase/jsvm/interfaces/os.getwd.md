Source: https://pocketbase.io/jsvm/interfaces/os.getwd.html

# Interface getwd

**Hierarchy**

- getwd

```ts
getwd(): string
```

Getwd returns an absolute path name corresponding to the current directory. If the current directory can be reached via multiple paths (due to symbolic links), Getwd may return any one of them.

On Unix platforms, if the environment variable PWD provides an absolute name, and it is a name of the current directory, it is returned.

**Returns string**
