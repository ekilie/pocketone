Source: https://pocketbase.io/jsvm/functions/_os.tempDir.html

# Function tempDir

```ts
tempDir(): string
```

TempDir returns the default directory to use for temporary files.

On Unix systems, it returns $TMPDIR if non-empty, else /tmp. On Windows, it uses GetTempPath, returning the first non-empty value from %TMP%, %TEMP%, %USERPROFILE%, or the Windows directory. On Plan 9, it returns /tmp.

The directory is neither guaranteed to exist nor have accessible permissions.

**Returns string**
