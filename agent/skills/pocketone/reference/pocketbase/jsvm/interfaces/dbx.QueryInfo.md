Source: https://pocketbase.io/jsvm/interfaces/dbx.QueryInfo.html

# Interface QueryInfo

QueryInfo represents a debug/info struct with exported SelectQuery fields.

**Hierarchy**

- QueryInfo

## Properties

### buildHook

```ts
buildHook: BuildHookFunc
```

### builder

```ts
builder: Builder
```

### context

```ts
context: context.Context
```

### distinct

```ts
distinct: boolean
```

### from

```ts
from: string[]
```

### groupBy

```ts
groupBy: string[]
```

### having

```ts
having: Expression
```

### join

```ts
join: JoinInfo[]
```

### limit

```ts
limit: number
```

### offset

```ts
offset: number
```

### orderBy

```ts
orderBy: string[]
```

### params

```ts
params: Params
```

### postFragment

```ts
postFragment: string
```

### preFragment

```ts
preFragment: string
```

### selectOption

```ts
selectOption: string
```

### selects

```ts
selects: string[]
```

### union

```ts
union: UnionInfo[]
```

### where

```ts
where: Expression
```
