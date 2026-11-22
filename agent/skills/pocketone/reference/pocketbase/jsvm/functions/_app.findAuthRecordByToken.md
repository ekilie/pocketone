Source: https://pocketbase.io/jsvm/functions/_app.findAuthRecordByToken.html

# Function findAuthRecordByToken

```ts
findAuthRecordByToken(token, ...validTypes): core.Record
```

FindAuthRecordByToken finds the auth record associated with the provided JWT (auth, file, verifyEmail, changeEmail, passwordReset types).

Optionally specify a list of validTypes to check tokens only from those types.

Returns an error if the JWT is invalid, expired or not associated to an auth collection record.

**Parameters**

- `token: string`
- `Rest ...validTypes: string[]`

**Returns core.Record**
