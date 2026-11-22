Source: https://pocketbase.io/jsvm/types/os._snLQTgO.html

# Type alias _snLQTgO

```ts
_snLQTgO: noWriteTo & os.File
```

fileWithoutWriteTo implements all the methods of *File other than WriteTo. This is used to permit WriteTo to call io.Copy without leading to a recursive call to WriteTo.
