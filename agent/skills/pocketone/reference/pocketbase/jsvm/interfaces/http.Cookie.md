Source: https://pocketbase.io/jsvm/interfaces/http.Cookie.html

# Interface Cookie

A Cookie represents an HTTP cookie as sent in the Set-Cookie header of an HTTP response or the Cookie header of an HTTP request.

See [https://tools.ietf.org/html/rfc6265](https://tools.ietf.org/html/rfc6265) for details.

**Hierarchy**

- Cookie

  - [Cookie](https://pocketbase.io/jsvm/classes/Cookie.html)

**Implemented by**

- [Cookie](https://pocketbase.io/jsvm/classes/Cookie.html)

## Methods

### string

```ts
string(): string
```

String returns the serialization of the cookie for use in a [Cookie] header (if only Name and Value are set) or a Set-Cookie response header (if other fields are set). If c is nil or c.Name is invalid, the empty string is returned.

**Returns string**

### valid

```ts
valid(): void
```

Valid reports whether the cookie is valid.

**Returns void**

## Properties

### domain

```ts
domain: string
```

### expires

```ts
expires: Time
```

### httpOnly

```ts
httpOnly: boolean
```

### maxAge

```ts
maxAge: number
```

MaxAge=0 means no 'Max-Age' attribute specified. MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0' MaxAge>0 means Max-Age attribute present and given in seconds

### name

```ts
name: string
```

### partitioned

```ts
partitioned: boolean
```

### path

```ts
path: string
```

### quoted

```ts
quoted: boolean
```

### raw

```ts
raw: string
```

### rawExpires

```ts
rawExpires: string
```

### sameSite

```ts
sameSite: SameSite
```

### secure

```ts
secure: boolean
```

### unparsed

```ts
unparsed: string[]
```

### value

```ts
value: string
```
