Source: https://pocketbase.io/jsvm/interfaces/os.link.html

# Interface link

**Hierarchy**

- link

```ts
link(oldname, newname): void
```

Link creates newname as a hard link to the oldname file. If there is an error, it will be of type *LinkError.

**Parameters**

- `oldname: string`
- `newname: string`

**Returns void**
