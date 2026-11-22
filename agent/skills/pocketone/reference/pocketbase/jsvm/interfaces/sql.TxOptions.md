Source: https://pocketbase.io/jsvm/interfaces/sql.TxOptions.html

# Interface TxOptions

TxOptions holds the transaction options to be used in [DB.BeginTx].

**Hierarchy**

- TxOptions

## Properties

### isolation

```ts
isolation: IsolationLevel
```

Isolation is the transaction isolation level. If zero, the driver or database's default level is used.

### readOnly

```ts
readOnly: boolean
```
