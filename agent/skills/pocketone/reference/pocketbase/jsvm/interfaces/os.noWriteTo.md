Source: https://pocketbase.io/jsvm/interfaces/os.noWriteTo.html

# Interface noWriteTo

noWriteTo can be embedded alongside another type to hide the WriteTo method of that other type.

**Hierarchy**

- noWriteTo

## Methods

### writeTo

```ts
writeTo(_arg0): number
```

WriteTo hides another WriteTo method. It should never be called.

**Parameters**

- `_arg0: io.Writer`

**Returns number**
