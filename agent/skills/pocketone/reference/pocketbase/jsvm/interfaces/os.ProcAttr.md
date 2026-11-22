Source: https://pocketbase.io/jsvm/interfaces/os.ProcAttr.html

# Interface ProcAttr

ProcAttr holds the attributes that will be applied to a new process started by StartProcess.

**Hierarchy**

- ProcAttr

## Properties

### dir

```ts
dir: string
```

If Dir is non-empty, the child changes into the directory before creating the process.

### env

```ts
env: string[]
```

If Env is non-nil, it gives the environment variables for the new process in the form returned by Environ. If it is nil, the result of Environ will be used.

### files

```ts
files: os.File[]
```

Files specifies the open files inherited by the new process. The first three entries correspond to standard input, standard output, and standard error. An implementation may support additional entries, depending on the underlying operating system. A nil entry corresponds to that file being closed when the process starts. On Unix systems, StartProcess will change these File values to blocking mode, which means that SetDeadline will stop working and calling Close will not interrupt a Read or Write.

### Optional sys

```ts
sys?: SysProcAttr
```

Operating system-specific process creation attributes. Note that setting this field means that your program may not execute properly or even compile on some operating systems.
