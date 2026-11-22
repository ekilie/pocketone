Source: https://pocketbase.io/jsvm/interfaces/os.newSyscallError.html

# Interface newSyscallError

**Hierarchy**

- newSyscallError

```ts
newSyscallError(syscall, err): void
```

NewSyscallError returns, as an error, a new [SyscallError] with the given system call name and error details. As a convenience, if err is nil, NewSyscallError returns nil.

**Parameters**

- `syscall: string`
- `err: Error`

**Returns void**
