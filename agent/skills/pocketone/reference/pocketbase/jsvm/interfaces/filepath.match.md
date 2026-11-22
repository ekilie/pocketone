Source: https://pocketbase.io/jsvm/interfaces/filepath.match.html

# Interface match

**Hierarchy**

- match

```ts
match(pattern, name): boolean
```

Match reports whether name matches the shell file name pattern. The pattern syntax is:

```
 pattern:
   { term }
 term:
   '*'         matches any sequence of non-Separator characters
   '?'         matches any single non-Separator character
   '[' [ '^' ] { character-range } ']'
               character class (must be non-empty)
   c           matches character c (c != '*', '?', '\\', '[')
   '\\' c      matches character c (except on Windows)

 character-range:
   c           matches character c (c != '\\', '-', ']')
   '\\' c      matches character c (except on Windows)
   lo '-' hi   matches character c for lo <= c <= hi
```

Path segments in the pattern must be separated by [Separator].

Match requires pattern to match all of name, not just a substring. The only possible returned error is [ErrBadPattern], when pattern is malformed.

On Windows, escaping is disabled. Instead, '\' is treated as path separator.

**Parameters**

- `pattern: string`
- `name: string`

**Returns boolean**
