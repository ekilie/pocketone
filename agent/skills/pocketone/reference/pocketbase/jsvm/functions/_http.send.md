Source: https://pocketbase.io/jsvm/functions/_http.send.html

# Function send

```ts
send(config): {     body: number[];     cookies: {         [key: string]: http.Cookie;     };     headers: {         [key: string]: string[];     };     json: any;     raw: string;     statusCode: number; }
```

Sends a single HTTP request.

Example:

```js
const res = $http.send({
    method:  "POST",
    url:     "https://example.com",
    body:    JSON.stringify({"title": "test"}),
    headers: { 'Content-Type': 'application/json' }
})

console.log(res.statusCode) // the response HTTP status code
console.log(res.headers)    // the response headers (eg. res.headers['X-Custom'][0])
console.log(res.cookies)    // the response cookies (eg. res.cookies.sessionId.value)
console.log(res.body)       // the response body as raw bytes slice
console.log(res.json)       // the response body as parsed json array or map
```

**Parameters**

- `config: {     body?: string | FormData;     data?: {         [key: string]: any;     };     headers?: {         [key: string]: string;     };     method?: string;     timeout?: number;     url: string; }`

**Returns {     body: number[];     cookies: {         [key: string]: http.Cookie;     };     headers: {         [key: string]: string[];     };     json: any;     raw: string;     statusCode: number; }**

**body: number[]**

**cookies: {     [key: string]: http.Cookie; }**

**[key: string]: http.Cookie**

**headers: {     [key: string]: string[]; }**

**[key: string]: string[]**

**json: any**

**raw: string**

**statusCode: number**
