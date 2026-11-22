Source: https://pocketbase.io/jsvm/interfaces/core.GetterFinder.html

# Interface GetterFinder

GetterFinder defines a field interface for registering custom field value getters.

**Hierarchy**

- GetterFinder

**Indexable**

```ts
[key: string]: any
```

## Methods

### findGetter

```ts
findGetter(key): GetterFunc
```

FindGetter returns a single field value getter function by performing pattern-like field matching using the specified key.

The key is usually just the field name but it could also contains "modifier" characters based on which you can perform custom get operations (ex. "description:excerpt" could be mapped to a function that will return an excerpt of the current field value).

Return nil if you want to fallback to the default field value setter.

**Parameters**

- `key: string`

**Returns GetterFunc**
