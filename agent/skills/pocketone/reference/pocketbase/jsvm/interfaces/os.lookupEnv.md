Source: https://pocketbase.io/jsvm/interfaces/os.lookupEnv.html

# Interface lookupEnv

**Hierarchy**

- lookupEnv

```ts
lookupEnv(key): [string, boolean]
```

LookupEnv retrieves the value of the environment variable named by the key. If the variable is present in the environment the value (which may be empty) is returned and the boolean is true. Otherwise the returned value will be empty and the boolean will be false.

**Parameters**

- `key: string`

**Returns [string, boolean]**
