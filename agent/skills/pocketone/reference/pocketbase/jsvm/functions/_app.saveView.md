Source: https://pocketbase.io/jsvm/functions/_app.saveView.html

# Function saveView

```ts
saveView(dangerousViewName, dangerousSelectQuery): void
```

SaveView creates (or updates already existing) persistent SQL view.

NB! Be aware that this method is vulnerable to SQL injection and its arguments must come only from trusted input!

**Parameters**

- `dangerousViewName: string`
- `dangerousSelectQuery: string`

**Returns void**
