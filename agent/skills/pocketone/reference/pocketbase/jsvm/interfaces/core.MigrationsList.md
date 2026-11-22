Source: https://pocketbase.io/jsvm/interfaces/core.MigrationsList.html

# Interface MigrationsList

MigrationsList defines a list with migration definitions

**Hierarchy**

- MigrationsList

## Methods

### add

```ts
add(m): void
```

Add adds adds an existing migration definition to the list.

If m.File is not provided, it will try to get the name from its .go file.

The list will be sorted automatically based on the migrations file name.

**Parameters**

- `m: Migration`

**Returns void**

### copy

```ts
copy(list): void
```

Copy copies all provided list migrations into the current one.

**Parameters**

- `list: MigrationsList`

**Returns void**

### item

```ts
item(index): Migration
```

Item returns a single migration from the list by its index.

**Parameters**

- `index: number`

**Returns Migration**

### items

```ts
items(): Migration[]
```

Items returns the internal migrations list slice.

**Returns Migration[]**

### register

```ts
register(up, down, ...optFilename): void
```

Register adds new migration definition to the list.

If optFilename is not provided, it will try to get the name from its .go file.

The list will be sorted automatically based on the migrations file name.

**Parameters**

- `up: ((txApp) => void)`
- `down: ((txApp) => void)`
- `Rest ...optFilename: string[]`

**Returns void**
