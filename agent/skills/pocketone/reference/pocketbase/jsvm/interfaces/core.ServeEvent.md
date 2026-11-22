Source: https://pocketbase.io/jsvm/interfaces/core.ServeEvent.html

# Interface ServeEvent

**Hierarchy**

- [_sHzIUqj](https://pocketbase.io/jsvm/types/core._sHzIUqj.html)

  - ServeEvent

## Methods

### next

```ts
next(): void
```

Next calls the next hook handler.

**Returns void**

## Properties

### app

```ts
app: App
```

### Optional certManager

```ts
certManager?: any
```

### extra

```ts
extra: _TygojaDict
```

Extra holds any dynamic data that could be passed between event handlers.

### installerFunc

```ts
installerFunc: ((app, systemSuperuser, baseURL) => void)
```

**Type declaration**

```ts
(app, systemSuperuser, baseURL): void
```

InstallerFunc is the "installer" function that is called after successful server tcp bind but only if there is no explicit superuser record created yet.

It runs in a separate goroutine and its default value is [apis.DefaultInstallerFunc].

It receives a system superuser record as argument that you can use to generate a short-lived auth token (e.g. systemSuperuser.NewStaticAuthToken(30 * time.Minute)) and concatenate it as query param for your installer page (if you are using the client-side SDKs, you can then load the token with pb.authStore.save(token) and perform any Web API request e.g. creating a new superuser).

Set it to nil if you want to skip the installer.

**Parameters**

- `app: App`
- `systemSuperuser: core.Record`
- `baseURL: string`

**Returns void**

### listener

```ts
listener: Listener
```

Listener allow specifying a custom network listener.

Leave it nil to use the default net.Listen("tcp", e.Server.Addr).

### Optional router

```ts
router?: Router<RequestEvent>
```

### Optional server

```ts
server?: Server
```

### startBannerFunc

```ts
startBannerFunc: ((baseURL) => void)
```

**Type declaration**

```ts
(baseURL): void
```

StartBannerFunc defines a custom console startup banner.

If not set, defaults to the system one that prints the current HTTP API and dashboard addresses.

Assign StartBannerFunc to a function with an empty body if you want to disable the console print entirely.

**Parameters**

- `baseURL: string`

**Returns void**

### uiExtensions

```ts
uiExtensions: UIExtension[]
```

### Todo

experimental

UIExtensions is a list with the superuser UI extensions.
