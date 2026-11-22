Source: https://pocketbase.io/jsvm/types/os._sOtguFd.html

# Type alias _sOtguFd

```ts
_sOtguFd: noReadFrom & os.File
```

fileWithoutReadFrom implements all the methods of *File other than ReadFrom. This is used to permit ReadFrom to call io.Copy without leading to a recursive call to ReadFrom.
