Source: https://pocketbase.io/jsvm/functions/_os.readFile.html

# Function readFile

```ts
readFile(name): string | number[]
```

ReadFile reads the named file and returns the contents. A successful call returns err == nil, not err == EOF. Because ReadFile reads the whole file, it does not treat an EOF from Read as an error to be reported. If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`

**Returns string | number[]**
