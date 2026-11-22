Source: https://pocketbase.io/jsvm/interfaces/url.URL.html

# Interface URL

A URL represents a parsed URL (technically, a URI reference).

The general form represented is:

```
  [scheme:][//[userinfo@]host][/]path[?query][#fragment]
```

URLs that do not start with a slash after the scheme are interpreted as:

```
  scheme:opaque[?query][#fragment]
```

The Host field contains the host and port subcomponents of the URL. When the port is present, it is separated from the host with a colon. When the host is an IPv6 address, it must be enclosed in square brackets: "[fe80::1]:80". The [net.JoinHostPort] function combines a host and port into a string suitable for the Host field, adding square brackets to the host when necessary.

Note that the Path field is stored in decoded form: /%47%6f%2f becomes /Go/. A consequence is that it is impossible to tell which slashes in the Path were slashes in the raw URL and which were %2f. This distinction is rarely important, but when it is, the code should use the [URL.EscapedPath] method, which preserves the original encoding of Path. The Fragment field is also stored in decoded form, use [URL.EscapedFragment] to retrieve the original encoding.

The [URL.String] method uses the [URL.EscapedPath] method to obtain the path.

**Hierarchy**

- URL

## Methods

### appendBinary

```ts
appendBinary(b): string | number[]
```

**Parameters**

- `b: string | number[]`

**Returns string | number[]**

### clone

```ts
clone(): URL
```

Clone creates a deep copy of the fields of the subject [URL].

**Returns URL**

### escapedFragment

```ts
escapedFragment(): string
```

EscapedFragment returns the escaped form of u.Fragment. In general there are multiple possible escaped forms of any fragment. EscapedFragment returns u.RawFragment when it is a valid escaping of u.Fragment. Otherwise EscapedFragment ignores u.RawFragment and computes an escaped form on its own. The [URL.String] method uses EscapedFragment to construct its result. In general, code should call EscapedFragment instead of reading u.RawFragment directly.

**Returns string**

### escapedPath

```ts
escapedPath(): string
```

EscapedPath returns the escaped form of u.Path. In general there are multiple possible escaped forms of any path. EscapedPath returns u.RawPath when it is a valid escaping of u.Path. Otherwise EscapedPath ignores u.RawPath and computes an escaped form on its own. The [URL.String] and [URL.RequestURI] methods use EscapedPath to construct their results. In general, code should call EscapedPath instead of reading u.RawPath directly.

**Returns string**

### hostname

```ts
hostname(): string
```

Hostname returns u.Host, stripping any valid port number if present.

If the result is enclosed in square brackets, as literal IPv6 addresses are, the square brackets are removed from the result.

**Returns string**

### isAbs

```ts
isAbs(): boolean
```

IsAbs reports whether the [URL] is absolute. Absolute means that it has a non-empty scheme.

**Returns boolean**

### joinPath

```ts
joinPath(...elem): URL
```

JoinPath returns a new [URL] with the provided path elements joined to any existing path and the resulting path cleaned of any ./ or ../ elements. Any sequences of multiple / characters will be reduced to a single /. Path elements must already be in escaped form, as produced by [PathEscape].

**Parameters**

- `Rest ...elem: string[]`

**Returns URL**

### marshalBinary

```ts
marshalBinary(): string | number[]
```

**Returns string | number[]**

### parse

```ts
parse(ref): URL
```

Parse parses a [URL] in the context of the receiver. The provided URL may be relative or absolute. Parse returns nil, err on parse failure, otherwise its return value is the same as [URL.ResolveReference].

**Parameters**

- `ref: string`

**Returns URL**

### port

```ts
port(): string
```

Port returns the port part of u.Host, without the leading colon.

If u.Host doesn't contain a valid numeric port, Port returns an empty string.

**Returns string**

### query

```ts
query(): Values
```

Query parses RawQuery and returns the corresponding values. It silently discards malformed value pairs. To check errors use [ParseQuery].

**Returns Values**

### redacted

```ts
redacted(): string
```

Redacted is like [URL.String] but replaces any password with "xxxxx". Only the password in u.User is redacted.

**Returns string**

### requestURI

```ts
requestURI(): string
```

RequestURI returns the encoded path?query or opaque?query string that would be used in an HTTP request for u.

**Returns string**

### resolveReference

```ts
resolveReference(ref): URL
```

ResolveReference resolves a URI reference to an absolute URI from an absolute base URI u, per RFC 3986 Section 5.2. The URI reference may be relative or absolute. ResolveReference always returns a new [URL] instance, even if the returned URL is identical to either the base or reference. If ref is an absolute URL, then ResolveReference ignores base and returns a copy of ref.

**Parameters**

- `ref: URL`

**Returns URL**

### string

```ts
string(): string
```

String reassembles the [URL] into a valid URL string. The general form of the result is one of:

```
 scheme:opaque?query#fragment
 scheme://userinfo@host/path?query#fragment
```

If u.Opaque is non-empty, String uses the first form; otherwise it uses the second form. Any non-ASCII characters in host are escaped. To obtain the path, String uses u.EscapedPath().

In the second form, the following rules apply:

```
  - if u.Scheme is empty, scheme: is omitted.
  - if u.User is nil, userinfo@ is omitted.
  - if u.Host is empty, host/ is omitted.
  - if u.Scheme and u.Host are empty and u.User is nil,
    the entire scheme://userinfo@host/ is omitted.
  - if u.Host is non-empty and u.Path begins with a /,
    the form host/path does not add its own /.
  - if u.RawQuery is empty, ?query is omitted.
  - if u.Fragment is empty, #fragment is omitted.
```

**Returns string**

### unmarshalBinary

```ts
unmarshalBinary(text): void
```

**Parameters**

- `text: string | number[]`

**Returns void**

## Properties

### forceQuery

```ts
forceQuery: boolean
```

ForceQuery indicates whether the original URL contained a query ('?') character. When set, the String method will include a trailing '?', even when RawQuery is empty.

### fragment

```ts
fragment: string
```

### host

```ts
host: string
```

### omitHost

```ts
omitHost: boolean
```

OmitHost indicates the URL has an empty host (authority). When set, the String method will not include the host when it is empty.

### opaque

```ts
opaque: string
```

### path

```ts
path: string
```

### rawFragment

```ts
rawFragment: string
```

RawFragment is an optional field containing an encoded fragment hint. See the EscapedFragment method for more details.

In general, code should call EscapedFragment instead of reading RawFragment.

### rawPath

```ts
rawPath: string
```

RawPath is an optional field containing an encoded path hint. See the EscapedPath method for more details.

In general, code should call EscapedPath instead of reading RawPath.

### rawQuery

```ts
rawQuery: string
```

RawQuery contains the encoded query values, without the initial '?'. Use URL.Query to decode the query.

### scheme

```ts
scheme: string
```

### Optional user

```ts
user?: Userinfo
```
