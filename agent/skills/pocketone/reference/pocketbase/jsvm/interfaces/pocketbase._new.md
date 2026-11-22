Source: https://pocketbase.io/jsvm/interfaces/pocketbase._new.html

# Interface _new

**Hierarchy**

- _new

```ts
_new(): pocketbase.PocketBase
```

New creates a new PocketBase instance with the default configuration. Use [NewWithConfig] if you want to provide a custom configuration.

Note that the application will not be initialized/bootstrapped yet, aka. DB connections, migrations, app settings, etc. will not be accessible. Everything will be initialized when [PocketBase.Start] is executed. If you want to initialize the application before calling [PocketBase.Start], then you'll have to manually call [PocketBase.Bootstrap].

**Returns pocketbase.PocketBase**
