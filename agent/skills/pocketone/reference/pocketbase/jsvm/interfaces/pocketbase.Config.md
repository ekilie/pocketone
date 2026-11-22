Source: https://pocketbase.io/jsvm/interfaces/pocketbase.Config.html

# Interface Config

Config is the PocketBase initialization config struct.

**Hierarchy**

- Config

## Properties

### auxMaxIdleConns

```ts
auxMaxIdleConns: number
```

### auxMaxOpenConns

```ts
auxMaxOpenConns: number
```

### dataMaxIdleConns

```ts
dataMaxIdleConns: number
```

### dataMaxOpenConns

```ts
dataMaxOpenConns: number
```

optional DB configurations

### dbConnect

```ts
dbConnect: DBConnectFunc
```

### defaultDataDir

```ts
defaultDataDir: string
```

### defaultDev

```ts
defaultDev: boolean
```

optional default values for the console flags

### defaultEncryptionEnv

```ts
defaultEncryptionEnv: string
```

### defaultQueryTimeout

```ts
defaultQueryTimeout: Duration
```

### hideStartBanner

```ts
hideStartBanner: boolean
```

hide the default console server info on app startup
