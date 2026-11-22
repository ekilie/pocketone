Source: https://pocketbase.io/jsvm/interfaces/mails.sendRecordOTP.html

# Interface sendRecordOTP

**Hierarchy**

- sendRecordOTP

```ts
sendRecordOTP(app, authRecord, otpId, pass): void
```

SendRecordOTP sends OTP email to the specified auth record.

This method will also update the "sentTo" field of the related OTP record to the mail sent To address (if the OTP exists and not already assigned).

**Parameters**

- `app: excludeHooks<App>`
- `authRecord: core.Record`
- `otpId: string`
- `pass: string`

**Returns void**
