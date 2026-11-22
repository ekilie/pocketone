Source: https://pocketbase.io/jsvm/interfaces/os.userConfigDir.html

# Interface userConfigDir

**Hierarchy**

- userConfigDir

```ts
userConfigDir(): string
```

UserConfigDir returns the default root directory to use for user-specific configuration data. Users should create their own application-specific subdirectory within this one and use that.

On Unix systems, it returns $XDG_CONFIG_HOME as specified by [https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html) if non-empty, else $HOME/.config. On Darwin, it returns $HOME/Library/Application Support. On Windows, it returns %AppData%. On Plan 9, it returns $home/lib.

If the location cannot be determined (for example, $HOME is not defined) or the path in $XDG_CONFIG_HOME is relative, then it will return an error.

**Returns string**
