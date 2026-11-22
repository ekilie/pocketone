Source: https://pocketbase.io/jsvm/classes/Cookie.html

# Class Cookie

A Cookie represents an HTTP cookie as sent in the Set-Cookie header of an HTTP response.

Example:

```js
routerAdd("POST", "/example", (c) => {
    c.setCookie(new Cookie({
        name:     "example_name",
        value:    "example_value",
        path:     "/",
        domain:   "example.com",
        maxAge:   10,
        secure:   true,
        httpOnly: true,
        sameSite: 3,
    }))

    return c.redirect(200, "/");
})
```

**Hierarchy**

- [http](https://pocketbase.io/jsvm/modules/http.html).[Cookie](https://pocketbase.io/jsvm/interfaces/http.Cookie.html)

  - Cookie

**Implements**

- [http](https://pocketbase.io/jsvm/modules/http.html).[Cookie](https://pocketbase.io/jsvm/interfaces/http.Cookie.html)

## Constructors

### constructor

```ts
new Cookie(options?): Cookie
```

**Parameters**

- `Optional options: Partial<http.Cookie>`

**Returns Cookie**

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
