Source: https://pocketbase.io/jsvm/interfaces/os.mkdir.html

# Interface mkdir

**Hierarchy**

- mkdir

```ts
mkdir(name, perm): void
```

Mkdir creates a new directory with the specified name and permission bits (before umask). If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`
- `perm: os.FileMode`

**Returns void**
