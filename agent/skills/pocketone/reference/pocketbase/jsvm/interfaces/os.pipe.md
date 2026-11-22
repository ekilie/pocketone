Source: https://pocketbase.io/jsvm/interfaces/os.pipe.html

# Interface pipe

**Hierarchy**

- pipe

```ts
pipe(): [os.File, os.File]
```

Pipe returns a connected pair of Files; reads from r return bytes written to w. It returns the files and an error, if any.

**Returns [os.File, os.File]**
