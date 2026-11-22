Source: https://pocketbase.io/jsvm/interfaces/multipart.Form.html

# Interface Form

Form is a parsed multipart form. Its File parts are stored either in memory or on disk, and are accessible via the [*FileHeader]'s Open method. Its Value parts are stored as strings. Both are keyed by field name.

**Hierarchy**

- Form

## Methods

### removeAll

```ts
removeAll(): void
```

RemoveAll removes any temporary files associated with a [Form].

**Returns void**

## Properties

### file

```ts
file: _TygojaDict
```

### value

```ts
value: _TygojaDict
```
