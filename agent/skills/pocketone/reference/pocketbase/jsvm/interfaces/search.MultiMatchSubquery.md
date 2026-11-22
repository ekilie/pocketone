Source: https://pocketbase.io/jsvm/interfaces/search.MultiMatchSubquery.html

# Interface MultiMatchSubquery

MultiMatchSubquery defines a multi-match record subquery expression.

**Hierarchy**

- MultiMatchSubquery

## Methods

### build

```ts
build(db, params): string
```

Build converts the expression into a SQL fragment.

Implements [dbx.Expression] interface.

**Parameters**

- `db: dbx.DB`
- `params: Params`

**Returns string**

## Properties

### fromTableAlias

```ts
fromTableAlias: string
```

### fromTableName

```ts
fromTableName: string
```

### joins

```ts
joins: Join[]
```

### params

```ts
params: Params
```

### targetTableAlias

```ts
targetTableAlias: string
```

### valueIdentifier

```ts
valueIdentifier: string
```
