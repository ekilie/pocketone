Source: https://pocketbase.io/jsvm/functions/_app.deleteView.html

# Function deleteView

```ts
deleteView(dangerousViewName): void
```

DeleteView drops the specified view name.

This method is a no-op if a view with the provided name doesn't exist.

NB! Be aware that this method is vulnerable to SQL injection and the "dangerousViewName" argument must come only from trusted input!

**Parameters**

- `dangerousViewName: string`

**Returns void**
