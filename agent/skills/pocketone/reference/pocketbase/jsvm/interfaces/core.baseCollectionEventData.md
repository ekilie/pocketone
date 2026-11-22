Source: https://pocketbase.io/jsvm/interfaces/core.baseCollectionEventData.html

# Interface baseCollectionEventData

**Hierarchy**

- baseCollectionEventData

## Methods

### tags

```ts
tags(): string[]
```

**Returns string[]**

### Todo

consider storing the original collection name and use that as a tag to avoid the ambiguity when the collection is being modified (#7613); for new collection also maybe return empty tags?

## Properties

### Optional collection

```ts
collection?: core.Collection
```
