Source: https://pocketbase.io/jsvm/interfaces/apis.defaultInstallerFunc.html

# Interface defaultInstallerFunc

**Hierarchy**

- defaultInstallerFunc

```ts
defaultInstallerFunc(app, systemSuperuser, baseURL): void
```

DefaultInstallerFunc is the default PocketBase installer function.

It will attempt to open a link in the browser (with a short-lived auth token for the systemSuperuser) to the installer UI so that users can create their own custom superuser record.

See [https://ganigeorgiev.com/pocketbase/discussions/5814](https://ganigeorgiev.com/pocketbase/discussions/5814).

**Parameters**

- `app: excludeHooks<App>`
- `systemSuperuser: core.Record`
- `baseURL: string`

**Returns void**
