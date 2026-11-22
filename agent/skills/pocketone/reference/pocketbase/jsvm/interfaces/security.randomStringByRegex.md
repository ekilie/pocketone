Source: https://pocketbase.io/jsvm/interfaces/security.randomStringByRegex.html

# Interface randomStringByRegex

**Hierarchy**

- randomStringByRegex

```ts
randomStringByRegex(pattern, ...optFlags): string
```

RandomStringByRegex generates a random string matching the regex pattern. If optFlags is not set, fallbacks to [syntax.Perl].

NB! While the source of the randomness comes from [crypto/rand] this method is not recommended to be used on its own in critical secure contexts because the generated length could vary too much on the used pattern and may not be as secure as simply calling [security.RandomString]. If you still insist on using it for such purposes, consider at least a large enough minimum length for the generated string, e.g. `[a-z0-9]{30}`.

This function is inspired by github.com/pipe01/revregexp, github.com/lucasjones/reggen and other similar packages.

**Parameters**

- `pattern: string`
- `Rest ...optFlags: Flags[]`

**Returns string**
