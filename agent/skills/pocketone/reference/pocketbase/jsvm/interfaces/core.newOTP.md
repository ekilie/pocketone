Source: https://pocketbase.io/jsvm/interfaces/core.newOTP.html

# Interface newOTP

**Hierarchy**

- newOTP

```ts
newOTP(app): OTP
```

NewOTP instantiates and returns a new blank *OTP model.

Example usage:

```
 otp := core.NewOTP(app)
 otp.SetRecordRef(user.Id)
 otp.SetCollectionRef(user.Collection().Id)
 otp.SetPassword(security.RandomStringWithAlphabet(6, "1234567890"))
 app.Save(otp)
```

**Parameters**

- `app: App`

**Returns OTP**
