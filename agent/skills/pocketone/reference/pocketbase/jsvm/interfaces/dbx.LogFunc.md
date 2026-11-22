Source: https://pocketbase.io/jsvm/interfaces/dbx.LogFunc.html

# Interface LogFunc

LogFunc logs a message for each SQL statement being executed. This method takes one or multiple parameters. If a single parameter is provided, it will be treated as the log message. If multiple parameters are provided, they will be passed to fmt.Sprintf() to generate the log message.

**Hierarchy**

- LogFunc

```ts
LogFunc(format, ...a): void
```

**Parameters**

- `format: string`
- `Rest ...a: {}[]`

**Returns void**
