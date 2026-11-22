Source: https://pocketbase.io/jsvm/interfaces/os.Process.html

# Interface Process

Process stores the information about a process created by [StartProcess].

**Hierarchy**

- Process

## Methods

### kill

```ts
kill(): void
```

Kill causes the [Process] to exit immediately. Kill does not wait until the Process has actually exited. This only kills the Process itself, not any other processes it may have started.

**Returns void**

### release

```ts
release(): void
```

Release releases any resources associated with the [Process] p, rendering it unusable in the future. Release only needs to be called if [Process.Wait] is not.

**Returns void**

### signal

```ts
signal(sig): void
```

Signal sends a signal to the [Process]. Sending [Interrupt] on Windows is not implemented.

**Parameters**

- `sig: os.Signal`

**Returns void**

### wait

```ts
wait(): ProcessState
```

Wait waits for the [Process] to exit, and then returns a ProcessState describing its status and an error, if any. Wait releases any resources associated with the Process. On most operating systems, the Process must be a child of the current process or an error will be returned.

**Returns ProcessState**

### withHandle

```ts
withHandle(f): void
```

WithHandle calls a supplied function f with a valid process handle as an argument. The handle is guaranteed to refer to process p until f returns, even if p terminates. This function cannot be used after [Process.Release] or [Process.Wait].

If process handles are not supported or a handle is not available, it returns [ErrNoHandle]. Currently, process handles are supported on Linux 5.4 or later (pidfd) and Windows.

**Parameters**

- `f: ((handle) => void)`

**Returns void**

## Properties

### pid

```ts
pid: number
```

Pid is the operating system process ID.
