Source: https://pocketbase.io/jsvm/functions/_os.getenv.html

# Function getenv

```ts
getenv(key): string
```

Getenv retrieves the value of the environment variable named by the key. It returns the value, which will be empty if the variable is not present. To distinguish between an empty value and an unset value, use [LookupEnv].

**Parameters**

- `key: string`

**Returns string**
