Source: https://pocketbase.io/jsvm/interfaces/search.ResolverResult.html

# Interface ResolverResult

ResolverResult defines a single FieldResolver.Resolve() successfully parsed result.

**Hierarchy**

- ResolverResult

## Properties

### afterBuild

```ts
afterBuild: ((expr) => Expression)
```

**Type declaration**

```ts
(expr): Expression
```

AfterBuild is an optional function that will be called after building and combining the result of both resolved operands/sides in a single expression.

**Parameters**

- `expr: Expression`

**Returns Expression**

### identifier

```ts
identifier: string
```

Identifier is the plain SQL identifier/column that will be used in the final db expression as left or right operand.

### Optional multiMatchSubQuery

```ts
multiMatchSubQuery?: MultiMatchSubquery
```

MultiMatchSubQuery is an optional sub query expression that will be added in addition to the combined ResolverResult expression during build.

### nullFallback

```ts
nullFallback: NullFallbackPreference
```

NullFallback specify the preference for how NULL or empty values should be resolved (default to "auto").

Set to NullFallbackDisabled to prevent any COALESCE or NULL fallbacks. Set to NullFallbackEnforced to prefer COALESCE or NULL fallbacks when needed.

### params

```ts
params: Params
```

Params is a map with db placeholder->value pairs that will be added to the query when building both resolved operands/sides in a single expression.
