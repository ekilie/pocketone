Source: https://pocketbase.io/jsvm/functions/_os.writeFile.html

# Function writeFile

```ts
writeFile(name, data, perm): void
```

WriteFile writes data to the named file, creating it if necessary. If the file does not exist, WriteFile creates it with permissions perm (before umask); otherwise WriteFile truncates it before writing, without changing permissions. Since WriteFile requires multiple system calls to complete, a failure mid-operation can leave the file in a partially written state.

**Parameters**

- `name: string`
- `data: string | number[]`
- `perm: os.FileMode`

**Returns void**
