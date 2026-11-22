Source: https://pocketbase.io/jsvm/interfaces/exec.Cmd.html

# Interface Cmd

Cmd represents an external command being prepared or run.

A Cmd cannot be reused after calling its [Cmd.Start], [Cmd.Run], [Cmd.Output], or [Cmd.CombinedOutput] methods.

**Hierarchy**

- Cmd

## Methods

### combinedOutput

```ts
combinedOutput(): string | number[]
```

CombinedOutput runs the command and returns its combined standard output and standard error.

**Returns string | number[]**

### environ

```ts
environ(): string[]
```

Environ returns a copy of the environment in which the command would be run as it is currently configured.

**Returns string[]**

### output

```ts
output(): string | number[]
```

Output runs the command and returns its standard output. Any returned error will usually be of type [*ExitError]. If c.Stderr was nil and the returned error is of type [*ExitError], Output populates the Stderr field of the returned error.

**Returns string | number[]**

### run

```ts
run(): void
```

Run starts the specified command and waits for it to complete.

The returned error is nil if the command runs, has no problems copying stdin, stdout, and stderr, and exits with a zero exit status.

If the command starts but does not complete successfully, the error is of type [*ExitError]. Other error types may be returned for other situations.

If the calling goroutine has locked the operating system thread with [runtime.LockOSThread] and modified any inheritable OS-level thread state (for example, Linux or Plan 9 name spaces), the new process will inherit the caller's thread state.

**Returns void**

### start

```ts
start(): void
```

Start starts the specified command but does not wait for it to complete.

If Start returns successfully, the c.Process field will be set.

After a successful call to Start the [Cmd.Wait] method must be called in order to release associated system resources.

**Returns void**

### stderrPipe

```ts
stderrPipe(): ReadCloser
```

StderrPipe returns a pipe that will be connected to the command's standard error when the command starts.

[Cmd.Wait] will close the pipe after seeing the command exit, so most callers need not close the pipe themselves. It is thus incorrect to call Wait before all reads from the pipe have completed. For the same reason, it is incorrect to use [Cmd.Run] when using StderrPipe. See the StdoutPipe example for idiomatic usage.

**Returns ReadCloser**

### stdinPipe

```ts
stdinPipe(): WriteCloser
```

StdinPipe returns a pipe that will be connected to the command's standard input when the command starts. The pipe will be closed automatically after [Cmd.Wait] sees the command exit. A caller need only call Close to force the pipe to close sooner. For example, if the command being run will not exit until standard input is closed, the caller must close the pipe.

**Returns WriteCloser**

### stdoutPipe

```ts
stdoutPipe(): ReadCloser
```

StdoutPipe returns a pipe that will be connected to the command's standard output when the command starts.

[Cmd.Wait] will close the pipe after seeing the command exit, so most callers need not close the pipe themselves. It is thus incorrect to call Wait before all reads from the pipe have completed. For the same reason, it is incorrect to call [Cmd.Run] when using StdoutPipe. See the example for idiomatic usage.

**Returns ReadCloser**

### string

```ts
string(): string
```

String returns a human-readable description of c. It is intended only for debugging. In particular, it is not suitable for use as input to a shell. The output of String may vary across Go releases.

**Returns string**

### wait

```ts
wait(): void
```

Wait waits for the command to exit and waits for any copying to stdin or copying from stdout or stderr to complete.

The command must have been started by [Cmd.Start].

The returned error is nil if the command runs, has no problems copying stdin, stdout, and stderr, and exits with a zero exit status.

If the command fails to run or doesn't complete successfully, the error is of type [*ExitError]. Other error types may be returned for I/O problems.

If any of c.Stdin, c.Stdout or c.Stderr are not an [*os.File], Wait also waits for the respective I/O loop copying to or from the process to complete.

Wait must not be called concurrently from multiple goroutines. A custom Cmd.Cancel function should not call Wait.

Wait releases any resources associated with the [Cmd].

**Returns void**

## Properties

### args

```ts
args: string[]
```

Args holds command line arguments, including the command as Args[0]. If the Args field is empty or nil, Run uses {Path}.

In typical use, both Path and Args are set by calling Command.

### cancel

```ts
cancel: (() => void)
```

**Type declaration**

```ts
(): void
```

If Cancel is non-nil, the command must have been created with CommandContext and Cancel will be called when the command's Context is done. By default, CommandContext sets Cancel to call the Kill method on the command's Process.

Typically a custom Cancel will send a signal to the command's Process, but it may instead take other actions to initiate cancellation, such as closing a stdin or stdout pipe or sending a shutdown request on a network socket.

If the command exits with a success status after Cancel is called, and Cancel does not return an error equivalent to os.ErrProcessDone, then Wait and similar methods will return a non-nil error: either an error wrapping the one returned by Cancel, or the error from the Context. (If the command exits with a non-success status, or Cancel returns an error that wraps os.ErrProcessDone, Wait and similar methods continue to return the command's usual exit status.)

If Cancel is set to nil, nothing will happen immediately when the command's Context is done, but a nonzero WaitDelay will still take effect. That may be useful, for example, to work around deadlocks in commands that do not support shutdown signals but are expected to always finish quickly.

Cancel will not be called if Start returns a non-nil error.

**Returns void**

### dir

```ts
dir: string
```

Dir specifies the working directory of the command. If Dir is the empty string, Run runs the command in the calling process's current directory.

On Unix systems, the value of Dir also determines the child process's PWD environment variable if not otherwise specified. A Unix process represents its working directory not by name but as an implicit reference to a node in the file tree. So, if the child process obtains its working directory by calling a function such as C's getcwd, which computes the canonical name by walking up the file tree, it will not recover the original value of Dir if that value was an alias involving symbolic links. However, if the child process calls Go's [os.Getwd] or GNU C's get_current_dir_name, and the value of PWD is an alias for the current directory, those functions will return the value of PWD, which matches the value of Dir.

### env

```ts
env: string[]
```

Env specifies the environment of the process. Each entry is of the form "key=value". If Env is nil, the new process uses the current process's environment. If Env contains duplicate environment keys, only the last value in the slice for each duplicate key is used. As a special case on Windows, SYSTEMROOT is always added if missing and not explicitly set to the empty string.

See also the Dir field, which may set PWD in the environment.

### err

```ts
err: Error
```

### extraFiles

```ts
extraFiles: os.File[]
```

ExtraFiles specifies additional open files to be inherited by the new process. It does not include standard input, standard output, or standard error. If non-nil, entry i becomes file descriptor 3+i.

ExtraFiles is not supported on Windows.

### path

```ts
path: string
```

Path is the path of the command to run.

This is the only field that must be set to a non-zero value. If Path is relative, it is evaluated relative to Dir.

### Optional process

```ts
process?: Process
```

Process is the underlying process, once started.

### Optional processState

```ts
processState?: ProcessState
```

ProcessState contains information about an exited process. If the process was started successfully, Wait or Run will populate its ProcessState when the command completes.

### stderr

```ts
stderr: io.Writer
```

### stdin

```ts
stdin: io.Reader
```

Stdin specifies the process's standard input.

If Stdin is nil, the process reads from the null device (os.DevNull).

If Stdin is an *os.File, the process's standard input is connected directly to that file.

Otherwise, during the execution of the command a separate goroutine reads from Stdin and delivers that data to the command over a pipe. In this case, Wait does not complete until the goroutine stops copying, either because it has reached the end of Stdin (EOF or a read error), or because writing to the pipe returned an error, or because a nonzero WaitDelay was set and expired.

Regardless of WaitDelay, Wait can block until a Read from Stdin completes. If you need to use a blocking io.Reader, use the StdinPipe method to get a pipe, copy from the Reader to the pipe, and arrange to close the Reader after Wait returns.

### stdout

```ts
stdout: io.Writer
```

Stdout and Stderr specify the process's standard output and error.

If either is nil, Run connects the corresponding file descriptor to the null device (os.DevNull).

If either is an *os.File, the corresponding output from the process is connected directly to that file.

Otherwise, during the execution of the command a separate goroutine reads from the process over a pipe and delivers that data to the corresponding Writer. In this case, Wait does not complete until the goroutine reaches EOF or encounters an error or a nonzero WaitDelay expires.

Regardless of WaitDelay, Wait can block until a Write to Stdout or Stderr completes. If you need to use a blocking io.Writer, use the StdoutPipe or StderrPipe method to get a pipe, copy from the pipe to the Writer, and arrange to close the Writer after Wait returns.

If Stdout and Stderr are the same writer, and have a type that can be compared with ==, at most one goroutine at a time will call Write.

### Optional sysProcAttr

```ts
sysProcAttr?: SysProcAttr
```

SysProcAttr holds optional, operating system-specific attributes. Run passes it to os.StartProcess as the os.ProcAttr's Sys field.

### waitDelay

```ts
waitDelay: Duration
```

If WaitDelay is non-zero, it bounds the time spent waiting on two sources of unexpected delay in Wait: a child process that fails to exit after the associated Context is canceled, and a child process that exits but leaves its I/O pipes unclosed.

The WaitDelay timer starts when either the associated Context is done or a call to Wait observes that the child process has exited, whichever occurs first. When the delay has elapsed, the command shuts down the child process and/or its I/O pipes.

If the child process has failed to exit — perhaps because it ignored or failed to receive a shutdown signal from a Cancel function, or because no Cancel function was set — then it will be terminated using os.Process.Kill.

Then, if the I/O pipes communicating with the child process are still open, those pipes are closed in order to unblock any goroutines currently blocked on Read or Write calls.

If pipes are closed due to WaitDelay, no Cancel call has occurred, and the command has otherwise exited with a successful status, Wait and similar methods will return ErrWaitDelay instead of nil.

If WaitDelay is zero (the default), I/O pipes will be read until EOF, which might not occur until orphaned subprocesses of the command have also closed their descriptors for the pipes.
