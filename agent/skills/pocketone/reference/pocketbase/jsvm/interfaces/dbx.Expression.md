Source: https://pocketbase.io/jsvm/interfaces/dbx.Expression.html

# Interface Expression

Expression represents a DB expression that can be embedded in a SQL statement.

**Hierarchy**

- Expression

**Indexable**

```ts
[key: string]: any
```

## Methods

### build

```ts
build(_arg0, _arg1): string
```

Build converts an expression into a SQL fragment. If the expression contains binding parameters, they will be added to the given Params.

**Parameters**

- `_arg0: dbx.DB`
- `_arg1: Params`

**Returns string**
