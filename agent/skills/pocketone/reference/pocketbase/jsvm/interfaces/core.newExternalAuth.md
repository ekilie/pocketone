Source: https://pocketbase.io/jsvm/interfaces/core.newExternalAuth.html

# Interface newExternalAuth

**Hierarchy**

- newExternalAuth

```ts
newExternalAuth(app): ExternalAuth
```

NewExternalAuth instantiates and returns a new blank *ExternalAuth model.

Example usage:

```
 ea := core.NewExternalAuth(app)
 ea.SetRecordRef(user.Id)
 ea.SetCollectionRef(user.Collection().Id)
 ea.SetProvider("google")
 ea.SetProviderId("...")
 app.Save(ea)
```

**Parameters**

- `app: App`

**Returns ExternalAuth**
