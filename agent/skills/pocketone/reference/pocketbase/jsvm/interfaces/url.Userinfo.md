Source: https://pocketbase.io/jsvm/interfaces/url.Userinfo.html

# Interface Userinfo

The Userinfo type is an immutable encapsulation of username and password details for a [URL]. An existing Userinfo value is guaranteed to have a username set (potentially empty, as allowed by RFC 2396), and optionally a password.

**Hierarchy**

- Userinfo

## Methods

### password

```ts
password(): [string, boolean]
```

Password returns the password in case it is set, and whether it is set.

**Returns [string, boolean]**

### string

```ts
string(): string
```

String returns the encoded userinfo information in the standard form of "username[:password]".

**Returns string**

### username

```ts
username(): string
```

Username returns the username.

**Returns string**
