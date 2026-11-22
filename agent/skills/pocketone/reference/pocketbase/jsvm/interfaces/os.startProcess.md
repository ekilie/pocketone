Source: https://pocketbase.io/jsvm/interfaces/os.startProcess.html

# Interface startProcess

**Hierarchy**

- startProcess

```ts
startProcess(name, argv, attr): Process
```

StartProcess starts a new process with the program, arguments and attributes specified by name, argv and attr. The argv slice will become [os.Args] in the new process, so it normally starts with the program name.

If the calling goroutine has locked the operating system thread with [runtime.LockOSThread] and modified any inheritable OS-level thread state (for example, Linux or Plan 9 name spaces), the new process will inherit the caller's thread state.

StartProcess is a low-level interface. The [os/exec] package provides higher-level interfaces.

If there is an error, it will be of type [*PathError].

**Parameters**

- `name: string`
- `argv: string[]`
- `attr: ProcAttr`

**Returns Process**
