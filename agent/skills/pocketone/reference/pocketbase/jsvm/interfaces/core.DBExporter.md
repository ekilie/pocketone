Source: https://pocketbase.io/jsvm/interfaces/core.DBExporter.html

# Interface DBExporter

DBExporter defines an interface for custom DB data export. Usually used as part of [App.Save].

**Hierarchy**

- DBExporter

**Indexable**

```ts
[key: string]: any
```

## Methods

### dbExport

```ts
dbExport(app): _TygojaDict
```

DBExport returns a key-value map with the data to be used when saving the struct in the database.

**Parameters**

- `app: App`

**Returns _TygojaDict**
