Source: https://pocketbase.io/jsvm/interfaces/core.MultiValuer.html

# Interface MultiValuer

MultiValuer defines a field interface that every multi-valued (eg. with MaxSelect) field has.

**Hierarchy**

- MultiValuer

**Indexable**

```ts
[key: string]: any
```

## Methods

### isMultiple

```ts
isMultiple(): boolean
```

IsMultiple checks whether the field is configured to support multiple or single values.

**Returns boolean**
