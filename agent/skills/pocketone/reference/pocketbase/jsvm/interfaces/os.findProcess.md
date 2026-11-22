Source: https://pocketbase.io/jsvm/interfaces/os.findProcess.html

# Interface findProcess

**Hierarchy**

- findProcess

```ts
findProcess(pid): Process
```

FindProcess looks for a running process by its pid.

The [Process] it returns can be used to obtain information about the underlying operating system process.

On Unix systems, FindProcess always succeeds and returns a Process for the given pid, regardless of whether the process exists. To test whether the process actually exists, see whether p.Signal(syscall.Signal(0)) reports an error.

**Parameters**

- `pid: number`

**Returns Process**
