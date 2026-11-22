Source: https://pocketbase.io/jsvm/functions/_app.dryRunView.html

# Function dryRunView

```ts
dryRunView(dangerousSelectQuery, sampleSize): DryRunViewResult
```

DryRunView executes the provided query by creating a temporary view collection and returning a sample of the resulting query records (if valid).

The same caveats from CreateViewFields apply here too.

NB! Be aware that this method is vulnerable to SQL injection and the "dangerousSelectQuery" argument must come only from trusted input!

**Parameters**

- `dangerousSelectQuery: string`
- `sampleSize: number`

**Returns DryRunViewResult**
