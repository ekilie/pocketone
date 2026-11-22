Source: https://pocketbase.io/jsvm/interfaces/os.ProcessState.html

# Interface ProcessState

ProcessState stores information about a process, as reported by Wait.

**Hierarchy**

- ProcessState

## Methods

### exitCode

```ts
exitCode(): number
```

ExitCode returns the exit code of the exited process, or -1 if the process hasn't exited or was terminated by a signal.

**Returns number**

### exited

```ts
exited(): boolean
```

Exited reports whether the program has exited. On Unix systems this reports true if the program exited due to calling exit, but false if the program terminated due to a signal.

**Returns boolean**

### pid

```ts
pid(): number
```

Pid returns the process id of the exited process.

**Returns number**

### string

```ts
string(): string
```

**Returns string**

### success

```ts
success(): boolean
```

Success reports whether the program exited successfully, such as with exit status 0 on Unix.

**Returns boolean**

### sys

```ts
sys(): any
```

Sys returns system-dependent exit information about the process. Convert it to the appropriate underlying type, such as [syscall.WaitStatus] on Unix, to access its contents.

**Returns any**

### sysUsage

```ts
sysUsage(): any
```

SysUsage returns system-dependent resource usage information about the exited process. Convert it to the appropriate underlying type, such as [*syscall.Rusage] on Unix, to access its contents. (On Unix, *syscall.Rusage matches struct rusage as defined in the getrusage(2) manual page.)

**Returns any**

### systemTime

```ts
systemTime(): Duration
```

SystemTime returns the system CPU time of the exited process and its children.

**Returns Duration**

### userTime

```ts
userTime(): Duration
```

UserTime returns the user CPU time of the exited process and its children.

**Returns Duration**
