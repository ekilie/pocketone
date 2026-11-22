Source: https://pocketbase.io/jsvm/interfaces/dbx.LikeExp.html

# Interface LikeExp

LikeExp represents a variant of LIKE expressions.

**Hierarchy**

- LikeExp

## Methods

### build

```ts
build(db, params): string
```

Build converts an expression into a SQL fragment.

**Parameters**

- `db: dbx.DB`
- `params: Params`

**Returns string**

### escape

```ts
escape(...chars): LikeExp
```

Escape specifies how a LIKE expression should be escaped. Each string at position 2i represents a special character and the string at position 2i+1 is the corresponding escaped version.

**Parameters**

- `Rest ...chars: string[]`

**Returns LikeExp**

### match

```ts
match(left, right): LikeExp
```

Match specifies whether to do wildcard matching on the left and/or right of given strings.

**Parameters**

- `left: boolean`
- `right: boolean`

**Returns LikeExp**

## Properties

### like

```ts
like: string
```

Like stores the LIKE operator. It can be "LIKE", "NOT LIKE". It may also be customized as something like "ILIKE".
