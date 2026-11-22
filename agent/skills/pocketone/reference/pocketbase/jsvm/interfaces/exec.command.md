Source: https://pocketbase.io/jsvm/interfaces/exec.command.html

# Interface command

**Hierarchy**

- command

```ts
command(name, ...arg): Cmd
```

Command returns the [Cmd] struct to execute the named program with the given arguments.

It sets only the Path and Args in the returned structure.

If name contains no path separators, Command uses [LookPath] to resolve name to a complete path if possible. Otherwise it uses name directly as Path.

The returned Cmd's Args field is constructed from the command name followed by the elements of arg, so arg should not include the command name itself. For example, Command("echo", "hello"). Args[0] is always name, not the possibly resolved Path.

On Windows, processes receive the whole command line as a single string and do their own parsing. Command combines and quotes Args into a command line string with an algorithm compatible with applications using CommandLineToArgvW (which is the most common way). Notable exceptions are msiexec.exe and cmd.exe (and thus, all batch files), which have a different unquoting algorithm. In these or other similar cases, you can do the quoting yourself and provide the full command line in SysProcAttr.CmdLine, leaving Args empty.

**Parameters**

- `name: string`
- `Rest ...arg: string[]`

**Returns Cmd**
