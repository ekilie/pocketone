Source: https://pocketbase.io/jsvm/interfaces/core.SMTPConfig.html

# Interface SMTPConfig

**Hierarchy**

- SMTPConfig

## Methods

### marshalJSON

```ts
marshalJSON(): string | number[]
```

MarshalJSON implements the [json.Marshaler] interface.

**Returns string | number[]**

### validate

```ts
validate(): void
```

Validate makes SMTPConfig validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### authMethod

```ts
authMethod: string
```

SMTP AUTH - PLAIN (default) or LOGIN

### enabled

```ts
enabled: boolean
```

### host

```ts
host: string
```

### localName

```ts
localName: string
```

LocalName is optional domain name or IP address used for the EHLO/HELO exchange (if not explicitly set, defaults to "localhost").

This is required only by some SMTP servers, such as Gmail SMTP-relay.

### password

```ts
password: string
```

### port

```ts
port: number
```

### tls

```ts
tls: boolean
```

Whether to enforce TLS encryption for the mail server connection.

When set to false StartTLS command is send, leaving the server to decide whether to upgrade the connection or not.

### username

```ts
username: string
```
