Source: https://pocketbase.io/jsvm/interfaces/os.truncate.html

# Interface truncate

**Hierarchy**

- truncate

```ts
truncate(name, size): void
```

Truncate changes the size of the named file. If the file is a symbolic link, it changes the size of the link's target. If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`
- `size: number`

**Returns void**
