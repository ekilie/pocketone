Source: https://pocketbase.io/jsvm/interfaces/os.userCacheDir.html

# Interface userCacheDir

**Hierarchy**

- userCacheDir

```ts
userCacheDir(): string
```

UserCacheDir returns the default root directory to use for user-specific cached data. Users should create their own application-specific subdirectory within this one and use that.

On Unix systems, it returns $XDG_CACHE_HOME as specified by [https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html) if non-empty, else $HOME/.cache. On Darwin, it returns $HOME/Library/Caches. On Windows, it returns %LocalAppData%. On Plan 9, it returns $home/lib/cache.

If the location cannot be determined (for example, $HOME is not defined) or the path in $XDG_CACHE_HOME is relative, then it will return an error.

**Returns string**
