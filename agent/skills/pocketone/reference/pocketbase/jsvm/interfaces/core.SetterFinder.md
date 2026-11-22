Source: https://pocketbase.io/jsvm/interfaces/core.SetterFinder.html

# Interface SetterFinder

SetterFinder defines a field interface for registering custom field value setters.

**Hierarchy**

- SetterFinder

**Indexable**

```ts
[key: string]: any
```

## Methods

### findSetter

```ts
findSetter(key): SetterFunc
```

FindSetter returns a single field value setter function by performing pattern-like field matching using the specified key.

The key is usually just the field name but it could also contains "modifier" characters based on which you can perform custom set operations (ex. "users+" could be mapped to a function that will append new user to the existing field value).

Return nil if you want to fallback to the default field value setter.

**Parameters**

- `key: string`

**Returns SetterFunc**
