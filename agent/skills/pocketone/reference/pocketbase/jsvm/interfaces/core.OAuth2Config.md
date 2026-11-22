Source: https://pocketbase.io/jsvm/interfaces/core.OAuth2Config.html

# Interface OAuth2Config

**Hierarchy**

- OAuth2Config

## Methods

### getProviderConfig

```ts
getProviderConfig(name): [OAuth2ProviderConfig, boolean]
```

GetProviderConfig returns the first OAuth2ProviderConfig that matches the specified name.

Returns false and zero config if no such provider is available in c.Providers.

**Parameters**

- `name: string`

**Returns [OAuth2ProviderConfig, boolean]**

### unmarshalJSON

```ts
unmarshalJSON(b): void
```

UnmarshalJSON implements the [json.Unmarshaler] interface.

The main difference from the standard unmarshalization is that instead of replacing the entire providers config slice, we ensure that partially submitted provider data (e.g. without clientSecret) is merged on per config level based on the provider name ([https://ganigeorgiev.com/pocketbase/discussions/7815](https://ganigeorgiev.com/pocketbase/discussions/7815)).

**Parameters**

- `b: string | number[]`

**Returns void**

### validate

```ts
validate(): void
```

Validate makes OAuth2Config validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### enabled

```ts
enabled: boolean
```

### mappedFields

```ts
mappedFields: OAuth2KnownFields
```

### providers

```ts
providers: OAuth2ProviderConfig[]
```
