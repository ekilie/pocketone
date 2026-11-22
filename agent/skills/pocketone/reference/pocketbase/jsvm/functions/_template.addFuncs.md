Source: https://pocketbase.io/jsvm/functions/_template.addFuncs.html

# Function addFuncs

```ts
addFuncs(funcs): Registry
```

AddFuncs registers new global template functions.

The key of each map entry is the function name that will be used in the templates. If a function with the map entry name already exists it will be replaced with the new one.

The value of each map entry is a function that must have either a single return value, or two return values of which the second has type error.

Example:

```
 r.AddFuncs(map[string]any{
   "toUpper": func(str string) string {
       return strings.ToUppser(str)
   },
   ...
 })
```

**Parameters**

- `funcs: _TygojaDict`

**Returns Registry**
