Source: https://pocketbase.io/jsvm/interfaces/os.userHomeDir.html

# Interface userHomeDir

**Hierarchy**

- userHomeDir

```ts
userHomeDir(): string
```

UserHomeDir returns the current user's home directory.

On Unix, including macOS, it returns the $HOME environment variable. On Windows, it returns %USERPROFILE%. On Plan 9, it returns the $home environment variable.

If the expected variable is not set in the environment, UserHomeDir returns either a platform-specific default value or a non-nil error.

**Returns string**
