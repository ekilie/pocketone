Source: https://pocketbase.io/jsvm/interfaces/core.newAuthOrigin.html

# Interface newAuthOrigin

**Hierarchy**

- newAuthOrigin

```ts
newAuthOrigin(app): AuthOrigin
```

NewAuthOrigin instantiates and returns a new blank *AuthOrigin model.

Example usage:

```
 origin := core.NewOrigin(app)
 origin.SetRecordRef(user.Id)
 origin.SetCollectionRef(user.Collection().Id)
 origin.SetFingerprint("...")
 app.Save(origin)
```

**Parameters**

- `app: App`

**Returns AuthOrigin**
