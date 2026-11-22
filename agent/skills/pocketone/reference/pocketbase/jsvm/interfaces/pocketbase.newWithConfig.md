Source: https://pocketbase.io/jsvm/interfaces/pocketbase.newWithConfig.html

# Interface newWithConfig

**Hierarchy**

- newWithConfig

```ts
newWithConfig(config): pocketbase.PocketBase
```

NewWithConfig creates a new PocketBase instance with the provided config.

Note that the application will not be initialized/bootstrapped yet, aka. DB connections, migrations, app settings, etc. will not be accessible. Everything will be initialized when [PocketBase.Start] is executed. If you want to initialize the application before calling [PocketBase.Start], then you'll have to manually call [PocketBase.Bootstrap].

**Parameters**

- `config: Config`

**Returns pocketbase.PocketBase**
