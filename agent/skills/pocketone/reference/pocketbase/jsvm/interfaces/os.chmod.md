Source: https://pocketbase.io/jsvm/interfaces/os.chmod.html

# Interface chmod

**Hierarchy**

- chmod

```ts
chmod(name, mode): void
```

Chmod changes the mode of the named file to mode. If the file is a symbolic link, it changes the mode of the link's target. If there is an error, it will be of type [*PathError].

A different subset of the mode bits are used, depending on the operating system.

On Unix, the mode's permission bits, [ModeSetuid], [ModeSetgid], and [ModeSticky] are used.

On Windows, only the 0o200 bit (owner writable) of mode is used; it controls whether the file's read-only attribute is set or cleared. The other bits are currently unused. For compatibility with Go 1.12 and earlier, use a non-zero mode. Use mode 0o400 for a read-only file and 0o600 for a readable+writable file.

On Plan 9, the mode's permission bits, [ModeAppend], [ModeExclusive], and [ModeTemporary] are used.

**Parameters**

- `name: string`
- `mode: os.FileMode`

**Returns void**
