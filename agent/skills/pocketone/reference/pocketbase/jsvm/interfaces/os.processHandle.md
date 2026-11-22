Source: https://pocketbase.io/jsvm/interfaces/os.processHandle.html

# Interface processHandle

processHandle holds an operating system handle to a process. This is only used on systems that support that concept, currently Linux and Windows. This maintains a reference count to the handle, and closes the handle when the reference drops to zero.

**Hierarchy**

- processHandle
