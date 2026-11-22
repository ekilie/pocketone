Source: https://pocketbase.io/jsvm/interfaces/os.noReadFrom.html

# Interface noReadFrom

noReadFrom can be embedded alongside another type to hide the ReadFrom method of that other type.

**Hierarchy**

- noReadFrom

## Methods

### readFrom

```ts
readFrom(_arg0): number
```

ReadFrom hides another ReadFrom method. It should never be called.

**Parameters**

- `_arg0: io.Reader`

**Returns number**
