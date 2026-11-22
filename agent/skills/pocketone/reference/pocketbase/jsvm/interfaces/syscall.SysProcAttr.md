Source: https://pocketbase.io/jsvm/interfaces/syscall.SysProcAttr.html

# Interface SysProcAttr

**Hierarchy**

- SysProcAttr

## Properties

### ambientCaps

```ts
ambientCaps: number[]
```

### cgroupFD

```ts
cgroupFD: number
```

### chroot

```ts
chroot: string
```

### cloneflags

```ts
cloneflags: number
```

### Optional credential

```ts
credential?: Credential
```

### ctty

```ts
ctty: number
```

### foreground

```ts
foreground: boolean
```

Foreground places the child process group in the foreground. This implies Setpgid. The Ctty field must be set to the descriptor of the controlling TTY. Unlike Setctty, in this case Ctty must be a descriptor number in the parent process.

### gidMappings

```ts
gidMappings: SysProcIDMap[]
```

### gidMappingsEnableSetgroups

```ts
gidMappingsEnableSetgroups: boolean
```

GidMappingsEnableSetgroups enabling setgroups syscall. If false, then setgroups syscall will be disabled for the child process. This parameter is no-op if GidMappings == nil. Otherwise for unprivileged users this should be set to false for mappings work.

### noctty

```ts
noctty: boolean
```

### pdeathsig

```ts
pdeathsig: syscall.Signal
```

Pdeathsig, if non-zero, is a signal that the kernel will send to the child process when the creating thread dies. Note that the signal is sent on thread termination, which may happen before process termination. There are more details at [https://go.dev/issue/27505](https://go.dev/issue/27505).

### pgid

```ts
pgid: number
```

### Optional pidFD

```ts
pidFD?: number
```

PidFD, if not nil, is used to store the pidfd of a child, if the functionality is supported by the kernel, or -1. Note *PidFD is changed only if the process starts successfully.

### ptrace

```ts
ptrace: boolean
```

Ptrace tells the child to call ptrace(PTRACE_TRACEME). Call runtime.LockOSThread before starting a process with this set, and don't call UnlockOSThread until done with PtraceSyscall calls.

### setctty

```ts
setctty: boolean
```

Setctty sets the controlling terminal of the child to file descriptor Ctty. Ctty must be a descriptor number in the child process: an index into ProcAttr.Files. This is only meaningful if Setsid is true.

### setpgid

```ts
setpgid: boolean
```

Setpgid sets the process group ID of the child to Pgid, or, if Pgid == 0, to the new child's process ID.

### setsid

```ts
setsid: boolean
```

### uidMappings

```ts
uidMappings: SysProcIDMap[]
```

### unshareflags

```ts
unshareflags: number
```

### useCgroupFD

```ts
useCgroupFD: boolean
```
