Source: https://pocketbase.io/jsvm/interfaces/os.newFile.html

# Interface newFile

**Hierarchy**

- newFile

```ts
newFile(fd, name): os.File
```

NewFile returns a new [File] with the given file descriptor and name. The returned value will be nil if fd is not a valid file descriptor.

NewFile's behavior differs on some platforms:

```
  - On Unix, if fd is in non-blocking mode, NewFile will attempt to return a pollable file.
  - On Windows, if fd is opened for asynchronous I/O (that is, [syscall.FILE_FLAG_OVERLAPPED]
    has been specified in the [syscall.CreateFile] call), NewFile will attempt to return a pollable
    file by associating fd with the Go runtime I/O completion port.
    The I/O operations will be performed synchronously if the association fails.
```

Only pollable files support [File.SetDeadline], [File.SetReadDeadline], and [File.SetWriteDeadline].

After passing it to NewFile, fd may become invalid under the same conditions described in the comments of [File.Fd], and the same constraints apply.

**Parameters**

- `fd: number`
- `name: string`

**Returns os.File**
