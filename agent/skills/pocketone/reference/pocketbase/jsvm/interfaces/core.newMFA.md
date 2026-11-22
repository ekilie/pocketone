Source: https://pocketbase.io/jsvm/interfaces/core.newMFA.html

# Interface newMFA

**Hierarchy**

- newMFA

```ts
newMFA(app): MFA
```

NewMFA instantiates and returns a new blank *MFA model.

Example usage:

```
 mfa := core.NewMFA(app)
 mfa.SetRecordRef(user.Id)
 mfa.SetCollectionRef(user.Collection().Id)
 mfa.SetMethod(core.MFAMethodPassword)
 app.Save(mfa)
```

**Parameters**

- `app: App`

**Returns MFA**
