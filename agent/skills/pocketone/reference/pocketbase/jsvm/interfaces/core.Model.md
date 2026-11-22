Source: https://pocketbase.io/jsvm/interfaces/core.Model.html

# Interface Model

Model defines an interface with common methods that all db models should have.

Note: for simplicity composite pk are not supported.

**Hierarchy**

- Model

**Indexable**

```ts
[key: string]: any
```

## Methods

### isNew

```ts
isNew(): boolean
```

**Returns boolean**

### lastSavedPK

```ts
lastSavedPK(): any
```

**Returns any**

### markAsNew

```ts
markAsNew(): void
```

**Returns void**

### markAsNotNew

```ts
markAsNotNew(): void
```

**Returns void**

### pk

```ts
pk(): any
```

**Returns any**

### tableName

```ts
tableName(): string
```

**Returns string**
