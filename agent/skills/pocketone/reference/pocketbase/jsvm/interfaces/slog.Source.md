Source: https://pocketbase.io/jsvm/interfaces/slog.Source.html

# Interface Source

Source describes the location of a line of source code.

**Hierarchy**

- Source

## Properties

### file

```ts
file: string
```

File and Line are the file name and line number (1-based) of the source line. These may be the empty string and zero, respectively, if not known.

### function

```ts
function: string
```

Function is the package path-qualified function name containing the source line. If non-empty, this string uniquely identifies a single function in the program. This may be the empty string if not known.

### line

```ts
line: number
```
