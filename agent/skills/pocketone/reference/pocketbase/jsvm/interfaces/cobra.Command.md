Source: https://pocketbase.io/jsvm/interfaces/cobra.Command.html

# Interface Command

Command is just that, a command for your application. E.g. 'go run ...' - 'run' is the command. Cobra requires you to define the usage and description as part of your command definition to ensure usability.

**Hierarchy**

- Command

  - [Command](https://pocketbase.io/jsvm/classes/Command.html)

**Implemented by**

- [Command](https://pocketbase.io/jsvm/classes/Command.html)

## Methods

### addCommand

```ts
addCommand(...cmds): void
```

AddCommand adds one or more commands to this parent command.

**Parameters**

- `Rest ...cmds: cobra.Command[]`

**Returns void**

### addGroup

```ts
addGroup(...groups): void
```

AddGroup adds one or more command groups to this parent command.

**Parameters**

- `Rest ...groups: Group[]`

**Returns void**

### allChildCommandsHaveGroup

```ts
allChildCommandsHaveGroup(): boolean
```

AllChildCommandsHaveGroup returns if all subcommands are assigned to a group

**Returns boolean**

### argsLenAtDash

```ts
argsLenAtDash(): number
```

ArgsLenAtDash will return the length of c.Flags().Args at the moment when a -- was found during args parsing.

**Returns number**

### calledAs

```ts
calledAs(): string
```

CalledAs returns the command name or alias that was used to invoke this command or an empty string if the command has not been called.

**Returns string**

### commandPath

```ts
commandPath(): string
```

CommandPath returns the full path to this command.

**Returns string**

### commandPathPadding

```ts
commandPathPadding(): number
```

CommandPathPadding return padding for the command path.

**Returns number**

### commands

```ts
commands(): cobra.Command[]
```

Commands returns a sorted slice of child commands.

**Returns cobra.Command[]**

### containsGroup

```ts
containsGroup(groupID): boolean
```

ContainsGroup return if groupID exists in the list of command groups.

**Parameters**

- `groupID: string`

**Returns boolean**

### context

```ts
context(): context.Context
```

Context returns underlying command context. If command was executed with ExecuteContext or the context was set with SetContext, the previously set context will be returned. Otherwise, nil is returned.

Notice that a call to Execute and ExecuteC will replace a nil context of a command with a context.Background, so a background context will be returned by Context after one of these functions has been called.

**Returns context.Context**

### debugFlags

```ts
debugFlags(): void
```

DebugFlags used to determine which flags have been assigned to which commands and which persist.

**Returns void**

### displayName

```ts
displayName(): string
```

DisplayName returns the name to display in help text. Returns command Name() If CommandDisplayNameAnnoation is not set

**Returns string**

### errOrStderr

```ts
errOrStderr(): io.Writer
```

ErrOrStderr returns output to stderr

**Returns io.Writer**

### errPrefix

```ts
errPrefix(): string
```

ErrPrefix return error message prefix for the command

**Returns string**

### execute

```ts
execute(): void
```

Execute uses the args (os.Args[1:] by default) and run through the command tree finding appropriate matches for commands and then corresponding flags.

**Returns void**

### executeC

```ts
executeC(): cobra.Command
```

ExecuteC executes the command.

**Returns cobra.Command**

### executeContext

```ts
executeContext(ctx): void
```

ExecuteContext is the same as Execute(), but sets the ctx on the command. Retrieve ctx by calling cmd.Context() inside your *Run lifecycle or ValidArgs functions.

**Parameters**

- `ctx: context.Context`

**Returns void**

### executeContextC

```ts
executeContextC(ctx): cobra.Command
```

ExecuteContextC is the same as ExecuteC(), but sets the ctx on the command. Retrieve ctx by calling cmd.Context() inside your *Run lifecycle or ValidArgs functions.

**Parameters**

- `ctx: context.Context`

**Returns cobra.Command**

### find

```ts
find(args): [cobra.Command, string[]]
```

Find the target command given the args and command tree Meant to be run on the highest node. Only searches down.

**Parameters**

- `args: string[]`

**Returns [cobra.Command, string[]]**

### flag

```ts
flag(name): any
```

Flag climbs up the command tree looking for matching flag.

**Parameters**

- `name: string`

**Returns any**

### flagErrorFunc

```ts
flagErrorFunc(): ((_arg0, _arg1) => void)
```

FlagErrorFunc returns either the function set by SetFlagErrorFunc for this command or a parent, or it returns a function which returns the original error.

**Returns ((_arg0, _arg1) => void)**

```ts
(_arg0, _arg1): void
```

FlagErrorFunc returns either the function set by SetFlagErrorFunc for this command or a parent, or it returns a function which returns the original error.

**Parameters**

- `_arg0: cobra.Command`
- `_arg1: Error`

**Returns void**

### flags

```ts
flags(): any
```

Flags returns the complete FlagSet that applies to this command (local and persistent declared here and by all parents).

**Returns any**

### genBashCompletion

```ts
genBashCompletion(w): void
```

GenBashCompletion generates bash completion file and writes to the passed writer.

**Parameters**

- `w: io.Writer`

**Returns void**

### genBashCompletionFile

```ts
genBashCompletionFile(filename): void
```

GenBashCompletionFile generates bash completion file.

**Parameters**

- `filename: string`

**Returns void**

### genBashCompletionFileV2

```ts
genBashCompletionFileV2(filename, includeDesc): void
```

GenBashCompletionFileV2 generates Bash completion version 2.

**Parameters**

- `filename: string`
- `includeDesc: boolean`

**Returns void**

### genBashCompletionV2

```ts
genBashCompletionV2(w, includeDesc): void
```

GenBashCompletionV2 generates Bash completion file version 2 and writes it to the passed writer.

**Parameters**

- `w: io.Writer`
- `includeDesc: boolean`

**Returns void**

### genFishCompletion

```ts
genFishCompletion(w, includeDesc): void
```

GenFishCompletion generates fish completion file and writes to the passed writer.

**Parameters**

- `w: io.Writer`
- `includeDesc: boolean`

**Returns void**

### genFishCompletionFile

```ts
genFishCompletionFile(filename, includeDesc): void
```

GenFishCompletionFile generates fish completion file.

**Parameters**

- `filename: string`
- `includeDesc: boolean`

**Returns void**

### genPowerShellCompletion

```ts
genPowerShellCompletion(w): void
```

GenPowerShellCompletion generates powershell completion file without descriptions and writes it to the passed writer.

**Parameters**

- `w: io.Writer`

**Returns void**

### genPowerShellCompletionFile

```ts
genPowerShellCompletionFile(filename): void
```

GenPowerShellCompletionFile generates powershell completion file without descriptions.

**Parameters**

- `filename: string`

**Returns void**

### genPowerShellCompletionFileWithDesc

```ts
genPowerShellCompletionFileWithDesc(filename): void
```

GenPowerShellCompletionFileWithDesc generates powershell completion file with descriptions.

**Parameters**

- `filename: string`

**Returns void**

### genPowerShellCompletionWithDesc

```ts
genPowerShellCompletionWithDesc(w): void
```

GenPowerShellCompletionWithDesc generates powershell completion file with descriptions and writes it to the passed writer.

**Parameters**

- `w: io.Writer`

**Returns void**

### genZshCompletion

```ts
genZshCompletion(w): void
```

GenZshCompletion generates zsh completion file including descriptions and writes it to the passed writer.

**Parameters**

- `w: io.Writer`

**Returns void**

### genZshCompletionFile

```ts
genZshCompletionFile(filename): void
```

GenZshCompletionFile generates zsh completion file including descriptions.

**Parameters**

- `filename: string`

**Returns void**

### genZshCompletionFileNoDesc

```ts
genZshCompletionFileNoDesc(filename): void
```

GenZshCompletionFileNoDesc generates zsh completion file without descriptions.

**Parameters**

- `filename: string`

**Returns void**

### genZshCompletionNoDesc

```ts
genZshCompletionNoDesc(w): void
```

GenZshCompletionNoDesc generates zsh completion file without descriptions and writes it to the passed writer.

**Parameters**

- `w: io.Writer`

**Returns void**

### getFlagCompletionFunc

```ts
getFlagCompletionFunc(flagName): [CompletionFunc, boolean]
```

GetFlagCompletionFunc returns the completion function for the given flag of the command, if available.

**Parameters**

- `flagName: string`

**Returns [CompletionFunc, boolean]**

### globalNormalizationFunc

```ts
globalNormalizationFunc(): ((f, name) => any)
```

GlobalNormalizationFunc returns the global normalization function or nil if it doesn't exist.

**Returns ((f, name) => any)**

```ts
(f, name): any
```

GlobalNormalizationFunc returns the global normalization function or nil if it doesn't exist.

**Parameters**

- `f: any`
- `name: string`

**Returns any**

### groups

```ts
groups(): Group[]
```

Groups returns a slice of child command groups.

**Returns Group[]**

### hasAlias

```ts
hasAlias(s): boolean
```

HasAlias determines if a given string is an alias of the command.

**Parameters**

- `s: string`

**Returns boolean**

### hasAvailableFlags

```ts
hasAvailableFlags(): boolean
```

HasAvailableFlags checks if the command contains any flags (local plus persistent from the entire structure) which are not hidden or deprecated.

**Returns boolean**

### hasAvailableInheritedFlags

```ts
hasAvailableInheritedFlags(): boolean
```

HasAvailableInheritedFlags checks if the command has flags inherited from its parent command which are not hidden or deprecated.

**Returns boolean**

### hasAvailableLocalFlags

```ts
hasAvailableLocalFlags(): boolean
```

HasAvailableLocalFlags checks if the command has flags specifically declared locally which are not hidden or deprecated.

**Returns boolean**

### hasAvailablePersistentFlags

```ts
hasAvailablePersistentFlags(): boolean
```

HasAvailablePersistentFlags checks if the command contains persistent flags which are not hidden or deprecated.

**Returns boolean**

### hasAvailableSubCommands

```ts
hasAvailableSubCommands(): boolean
```

HasAvailableSubCommands determines if a command has available sub commands that need to be shown in the usage/help default template under 'available commands'.

**Returns boolean**

### hasExample

```ts
hasExample(): boolean
```

HasExample determines if the command has example.

**Returns boolean**

### hasFlags

```ts
hasFlags(): boolean
```

HasFlags checks if the command contains any flags (local plus persistent from the entire structure).

**Returns boolean**

### hasHelpSubCommands

```ts
hasHelpSubCommands(): boolean
```

HasHelpSubCommands determines if a command has any available 'help' sub commands that need to be shown in the usage/help default template under 'additional help topics'.

**Returns boolean**

### hasInheritedFlags

```ts
hasInheritedFlags(): boolean
```

HasInheritedFlags checks if the command has flags inherited from its parent command.

**Returns boolean**

### hasLocalFlags

```ts
hasLocalFlags(): boolean
```

HasLocalFlags checks if the command has flags specifically declared locally.

**Returns boolean**

### hasParent

```ts
hasParent(): boolean
```

HasParent determines if the command is a child command.

**Returns boolean**

### hasPersistentFlags

```ts
hasPersistentFlags(): boolean
```

HasPersistentFlags checks if the command contains persistent flags.

**Returns boolean**

### hasSubCommands

```ts
hasSubCommands(): boolean
```

HasSubCommands determines if the command has children commands.

**Returns boolean**

### help

```ts
help(): void
```

Help puts out the help for the command. Used when a user calls help [command]. Can be defined by user by overriding HelpFunc.

**Returns void**

### helpFunc

```ts
helpFunc(): ((_arg0, _arg1) => void)
```

HelpFunc returns either the function set by SetHelpFunc for this command or a parent, or it returns a function with default help behavior.

**Returns ((_arg0, _arg1) => void)**

```ts
(_arg0, _arg1): void
```

HelpFunc returns either the function set by SetHelpFunc for this command or a parent, or it returns a function with default help behavior.

**Parameters**

- `_arg0: cobra.Command`
- `_arg1: string[]`

**Returns void**

### helpTemplate

```ts
helpTemplate(): string
```

HelpTemplate return help template for the command. This function is kept for backwards-compatibility reasons.

**Returns string**

### inOrStdin

```ts
inOrStdin(): io.Reader
```

InOrStdin returns input to stdin

**Returns io.Reader**

### inheritedFlags

```ts
inheritedFlags(): any
```

InheritedFlags returns all flags which were inherited from parent commands. This function does not modify the flags of the current command, it's purpose is to return the current state.

**Returns any**

### initDefaultCompletionCmd

```ts
initDefaultCompletionCmd(...args): void
```

InitDefaultCompletionCmd adds a default 'completion' command to c. This function will do nothing if any of the following is true: 1- the feature has been explicitly disabled by the program, 2- c has no subcommands (to avoid creating one), 3- c already has a 'completion' command provided by the program.

**Parameters**

- `Rest ...args: string[]`

**Returns void**

### initDefaultHelpCmd

```ts
initDefaultHelpCmd(): void
```

InitDefaultHelpCmd adds default help command to c. It is called automatically by executing the c or by calling help and usage. If c already has help command or c has no subcommands, it will do nothing.

**Returns void**

### initDefaultHelpFlag

```ts
initDefaultHelpFlag(): void
```

InitDefaultHelpFlag adds default help flag to c. It is called automatically by executing the c or by calling help and usage. If c already has help flag, it will do nothing.

**Returns void**

### initDefaultVersionFlag

```ts
initDefaultVersionFlag(): void
```

InitDefaultVersionFlag adds default version flag to c. It is called automatically by executing the c. If c already has a version flag, it will do nothing. If c.Version is empty, it will do nothing.

**Returns void**

### isAdditionalHelpTopicCommand

```ts
isAdditionalHelpTopicCommand(): boolean
```

IsAdditionalHelpTopicCommand determines if a command is an additional help topic command; additional help topic command is determined by the fact that it is NOT runnable/hidden/deprecated, and has no sub commands that are runnable/hidden/deprecated. Concrete example: [https://github.com/spf13/cobra/issues/393#issuecomment-282741924](https://github.com/spf13/cobra/issues/393#issuecomment-282741924).

**Returns boolean**

### isAvailableCommand

```ts
isAvailableCommand(): boolean
```

IsAvailableCommand determines if a command is available as a non-help command (this includes all non deprecated/hidden commands).

**Returns boolean**

### localFlags

```ts
localFlags(): any
```

LocalFlags returns the local FlagSet specifically set in the current command. This function does not modify the flags of the current command, it's purpose is to return the current state.

**Returns any**

### localNonPersistentFlags

```ts
localNonPersistentFlags(): any
```

LocalNonPersistentFlags are flags specific to this command which will NOT persist to subcommands. This function does not modify the flags of the current command, it's purpose is to return the current state.

**Returns any**

### markFlagCustom

```ts
markFlagCustom(name, f): void
```

MarkFlagCustom adds the BashCompCustom annotation to the named flag, if it exists. The bash completion script will call the bash function f for the flag.

This will only work for bash completion. It is recommended to instead use c.RegisterFlagCompletionFunc(...) which allows to register a Go function which will work across all shells.

**Parameters**

- `name: string`
- `f: string`

**Returns void**

### markFlagDirname

```ts
markFlagDirname(name): void
```

MarkFlagDirname instructs the various shell completion implementations to limit completions for the named flag to directory names.

**Parameters**

- `name: string`

**Returns void**

### markFlagFilename

```ts
markFlagFilename(name, ...extensions): void
```

MarkFlagFilename instructs the various shell completion implementations to limit completions for the named flag to the specified file extensions.

**Parameters**

- `name: string`
- `Rest ...extensions: string[]`

**Returns void**

### markFlagRequired

```ts
markFlagRequired(name): void
```

MarkFlagRequired instructs the various shell completion implementations to prioritize the named flag when performing completion, and causes your command to report an error if invoked without the flag.

**Parameters**

- `name: string`

**Returns void**

### markFlagsMutuallyExclusive

```ts
markFlagsMutuallyExclusive(...flagNames): void
```

MarkFlagsMutuallyExclusive marks the given flags with annotations so that Cobra errors if the command is invoked with more than one flag from the given set of flags.

**Parameters**

- `Rest ...flagNames: string[]`

**Returns void**

### markFlagsOneRequired

```ts
markFlagsOneRequired(...flagNames): void
```

MarkFlagsOneRequired marks the given flags with annotations so that Cobra errors if the command is invoked without at least one flag from the given set of flags.

**Parameters**

- `Rest ...flagNames: string[]`

**Returns void**

### markFlagsRequiredTogether

```ts
markFlagsRequiredTogether(...flagNames): void
```

MarkFlagsRequiredTogether marks the given flags with annotations so that Cobra errors if the command is invoked with a subset (but not all) of the given flags.

**Parameters**

- `Rest ...flagNames: string[]`

**Returns void**

### markPersistentFlagDirname

```ts
markPersistentFlagDirname(name): void
```

MarkPersistentFlagDirname instructs the various shell completion implementations to limit completions for the named persistent flag to directory names.

**Parameters**

- `name: string`

**Returns void**

### markPersistentFlagFilename

```ts
markPersistentFlagFilename(name, ...extensions): void
```

MarkPersistentFlagFilename instructs the various shell completion implementations to limit completions for the named persistent flag to the specified file extensions.

**Parameters**

- `name: string`
- `Rest ...extensions: string[]`

**Returns void**

### markPersistentFlagRequired

```ts
markPersistentFlagRequired(name): void
```

MarkPersistentFlagRequired instructs the various shell completion implementations to prioritize the named persistent flag when performing completion, and causes your command to report an error if invoked without the flag.

**Parameters**

- `name: string`

**Returns void**

### markZshCompPositionalArgumentFile

```ts
markZshCompPositionalArgumentFile(argPosition, ...patterns): void
```

MarkZshCompPositionalArgumentFile only worked for zsh and its behavior was not consistent with Bash completion. It has therefore been disabled. Instead, when no other completion is specified, file completion is done by default for every argument. One can disable file completion on a per-argument basis by using ValidArgsFunction and ShellCompDirectiveNoFileComp. To achieve file extension filtering, one can use ValidArgsFunction and ShellCompDirectiveFilterFileExt.

Deprecated

**Parameters**

- `argPosition: number`
- `Rest ...patterns: string[]`

**Returns void**

### markZshCompPositionalArgumentWords

```ts
markZshCompPositionalArgumentWords(argPosition, ...words): void
```

MarkZshCompPositionalArgumentWords only worked for zsh. It has therefore been disabled. To achieve the same behavior across all shells, one can use ValidArgs (for the first argument only) or ValidArgsFunction for any argument (can include the first one also).

Deprecated

**Parameters**

- `argPosition: number`
- `Rest ...words: string[]`

**Returns void**

### name

```ts
name(): string
```

Name returns the command's name: the first word in the use line.

**Returns string**

### nameAndAliases

```ts
nameAndAliases(): string
```

NameAndAliases returns a list of the command name and all aliases

**Returns string**

### namePadding

```ts
namePadding(): number
```

NamePadding returns padding for the name.

**Returns number**

### nonInheritedFlags

```ts
nonInheritedFlags(): any
```

NonInheritedFlags returns all flags which were not inherited from parent commands. This function does not modify the flags of the current command, it's purpose is to return the current state.

**Returns any**

### outOrStderr

```ts
outOrStderr(): io.Writer
```

OutOrStderr returns output to stderr

**Returns io.Writer**

### outOrStdout

```ts
outOrStdout(): io.Writer
```

OutOrStdout returns output to stdout.

**Returns io.Writer**

### parent

```ts
parent(): cobra.Command
```

Parent returns a commands parent command.

**Returns cobra.Command**

### parseFlags

```ts
parseFlags(args): void
```

ParseFlags parses persistent flag tree and local flags.

**Parameters**

- `args: string[]`

**Returns void**

### persistentFlags

```ts
persistentFlags(): any
```

PersistentFlags returns the persistent FlagSet specifically set in the current command.

**Returns any**

### print

```ts
print(...i): void
```

Print is a convenience method to Print to the defined output, fallback to Stderr if not set.

**Parameters**

- `Rest ...i: {}[]`

**Returns void**

### printErr

```ts
printErr(...i): void
```

PrintErr is a convenience method to Print to the defined Err output, fallback to Stderr if not set.

**Parameters**

- `Rest ...i: {}[]`

**Returns void**

### printErrf

```ts
printErrf(format, ...i): void
```

PrintErrf is a convenience method to Printf to the defined Err output, fallback to Stderr if not set.

**Parameters**

- `format: string`
- `Rest ...i: {}[]`

**Returns void**

### printErrln

```ts
printErrln(...i): void
```

PrintErrln is a convenience method to Println to the defined Err output, fallback to Stderr if not set.

**Parameters**

- `Rest ...i: {}[]`

**Returns void**

### printf

```ts
printf(format, ...i): void
```

Printf is a convenience method to Printf to the defined output, fallback to Stderr if not set.

**Parameters**

- `format: string`
- `Rest ...i: {}[]`

**Returns void**

### println

```ts
println(...i): void
```

Println is a convenience method to Println to the defined output, fallback to Stderr if not set.

**Parameters**

- `Rest ...i: {}[]`

**Returns void**

### registerFlagCompletionFunc

```ts
registerFlagCompletionFunc(flagName, f): void
```

RegisterFlagCompletionFunc should be called to register a function to provide completion for a flag.

You can use pre-defined completion functions such as [FixedCompletions] or [NoFileCompletions], or you can define your own.

**Parameters**

- `flagName: string`
- `f: CompletionFunc`

**Returns void**

### removeCommand

```ts
removeCommand(...cmds): void
```

RemoveCommand removes one or more commands from a parent command.

**Parameters**

- `Rest ...cmds: cobra.Command[]`

**Returns void**

### resetCommands

```ts
resetCommands(): void
```

ResetCommands delete parent, subcommand and help command from c.

**Returns void**

### resetFlags

```ts
resetFlags(): void
```

ResetFlags deletes all flags from command.

**Returns void**

### root

```ts
root(): cobra.Command
```

Root finds root command.

**Returns cobra.Command**

### runnable

```ts
runnable(): boolean
```

Runnable determines if the command is itself runnable.

**Returns boolean**

### setArgs

```ts
setArgs(a): void
```

SetArgs sets arguments for the command. It is set to os.Args[1:] by default, if desired, can be overridden particularly useful when testing.

**Parameters**

- `a: string[]`

**Returns void**

### setCompletionCommandGroupID

```ts
setCompletionCommandGroupID(groupID): void
```

SetCompletionCommandGroupID sets the group id of the completion command.

**Parameters**

- `groupID: string`

**Returns void**

### setContext

```ts
setContext(ctx): void
```

SetContext sets context for the command. This context will be overwritten by Command.ExecuteContext or Command.ExecuteContextC.

**Parameters**

- `ctx: context.Context`

**Returns void**

### setErr

```ts
setErr(newErr): void
```

SetErr sets the destination for error messages. If newErr is nil, os.Stderr is used.

**Parameters**

- `newErr: io.Writer`

**Returns void**

### setErrPrefix

```ts
setErrPrefix(s): void
```

SetErrPrefix sets error message prefix to be used. Application can use it to set custom prefix.

**Parameters**

- `s: string`

**Returns void**

### setFlagErrorFunc

```ts
setFlagErrorFunc(f): void
```

SetFlagErrorFunc sets a function to generate an error when flag parsing fails.

**Parameters**

- `f: ((_arg0, _arg1) => void)`

**Returns void**

### setGlobalNormalizationFunc

```ts
setGlobalNormalizationFunc(n): void
```

SetGlobalNormalizationFunc sets a normalization function to all flag sets and also to child commands. The user should not have a cyclic dependency on commands.

**Parameters**

- `n: ((f, name) => any)`

**Returns void**

### setHelpCommand

```ts
setHelpCommand(cmd): void
```

SetHelpCommand sets help command.

**Parameters**

- `cmd: cobra.Command`

**Returns void**

### setHelpCommandGroupID

```ts
setHelpCommandGroupID(groupID): void
```

SetHelpCommandGroupID sets the group id of the help command.

**Parameters**

- `groupID: string`

**Returns void**

### setHelpFunc

```ts
setHelpFunc(f): void
```

SetHelpFunc sets help function. Can be defined by Application.

**Parameters**

- `f: ((_arg0, _arg1) => void)`

**Returns void**

### setHelpTemplate

```ts
setHelpTemplate(s): void
```

SetHelpTemplate sets help template to be used. Application can use it to set custom template.

**Parameters**

- `s: string`

**Returns void**

### setIn

```ts
setIn(newIn): void
```

SetIn sets the source for input data If newIn is nil, os.Stdin is used.

**Parameters**

- `newIn: io.Reader`

**Returns void**

### setOut

```ts
setOut(newOut): void
```

SetOut sets the destination for usage messages. If newOut is nil, os.Stdout is used.

**Parameters**

- `newOut: io.Writer`

**Returns void**

### setOutput

```ts
setOutput(output): void
```

SetOutput sets the destination for usage and error messages. If output is nil, os.Stderr is used.

Deprecated: Use SetOut and/or SetErr instead

**Parameters**

- `output: io.Writer`

**Returns void**

### setUsageFunc

```ts
setUsageFunc(f): void
```

SetUsageFunc sets usage function. Usage can be defined by application.

**Parameters**

- `f: ((_arg0) => void)`

**Returns void**

### setUsageTemplate

```ts
setUsageTemplate(s): void
```

SetUsageTemplate sets usage template. Can be defined by Application.

**Parameters**

- `s: string`

**Returns void**

### setVersionTemplate

```ts
setVersionTemplate(s): void
```

SetVersionTemplate sets version template to be used. Application can use it to set custom template.

**Parameters**

- `s: string`

**Returns void**

### suggestionsFor

```ts
suggestionsFor(typedName): string[]
```

SuggestionsFor provides suggestions for the typedName.

**Parameters**

- `typedName: string`

**Returns string[]**

### traverse

```ts
traverse(args): [cobra.Command, string[]]
```

Traverse the command tree to find the command, and parse args for each parent.

**Parameters**

- `args: string[]`

**Returns [cobra.Command, string[]]**

### usage

```ts
usage(): void
```

Usage puts out the usage for the command. Used when a user provides invalid input. Can be defined by user by overriding UsageFunc.

**Returns void**

### usageFunc

```ts
usageFunc(): ((_arg0) => void)
```

UsageFunc returns either the function set by SetUsageFunc for this command or a parent, or it returns a default usage function.

**Returns ((_arg0) => void)**

```ts
(_arg0): void
```

UsageFunc returns either the function set by SetUsageFunc for this command or a parent, or it returns a default usage function.

**Parameters**

- `_arg0: cobra.Command`

**Returns void**

### usagePadding

```ts
usagePadding(): number
```

UsagePadding return padding for the usage.

**Returns number**

### usageString

```ts
usageString(): string
```

UsageString returns usage string.

**Returns string**

### usageTemplate

```ts
usageTemplate(): string
```

UsageTemplate returns usage template for the command. This function is kept for backwards-compatibility reasons.

**Returns string**

### useLine

```ts
useLine(): string
```

UseLine puts out the full usage for a given command (including parents).

**Returns string**

### validateArgs

```ts
validateArgs(args): void
```

**Parameters**

- `args: string[]`

**Returns void**

### validateFlagGroups

```ts
validateFlagGroups(): void
```

ValidateFlagGroups validates the mutuallyExclusive/oneRequired/requiredAsGroup logic and returns the first error encountered.

**Returns void**

### validateRequiredFlags

```ts
validateRequiredFlags(): void
```

ValidateRequiredFlags validates all required flags are present and returns an error otherwise

**Returns void**

### versionTemplate

```ts
versionTemplate(): string
```

VersionTemplate return version template for the command. This function is kept for backwards-compatibility reasons.

**Returns string**

### visitParents

```ts
visitParents(fn): void
```

VisitParents visits all parents of the command and invokes fn on each parent.

**Parameters**

- `fn: ((_arg0) => void)`

**Returns void**

## Properties

### aliases

```ts
aliases: string[]
```

Aliases is an array of aliases that can be used instead of the first word in Use.

### annotations

```ts
annotations: _TygojaDict
```

Annotations are key/value pairs that can be used by applications to identify or group commands or set special options.

### argAliases

```ts
argAliases: string[]
```

ArgAliases is List of aliases for ValidArgs. These are not suggested to the user in the shell completion, but accepted if entered manually.

### args

```ts
args: PositionalArgs
```

Expected arguments

### bashCompletionFunction

```ts
bashCompletionFunction: string
```

BashCompletionFunction is custom bash functions used by the legacy bash autocompletion generator. For portability with other shells, it is recommended to instead use ValidArgsFunction

### completionOptions

```ts
completionOptions: CompletionOptions
```

CompletionOptions is a set of options to control the handling of shell completion

### deprecated

```ts
deprecated: string
```

Deprecated defines, if this command is deprecated and should print this string when used.

### disableAutoGenTag

```ts
disableAutoGenTag: boolean
```

DisableAutoGenTag defines, if gen tag ("Auto generated by spf13/cobra...") will be printed by generating docs for this command.

### disableFlagParsing

```ts
disableFlagParsing: boolean
```

DisableFlagParsing disables the flag parsing. If this is true all flags will be passed to the command as arguments.

### disableFlagsInUseLine

```ts
disableFlagsInUseLine: boolean
```

DisableFlagsInUseLine will disable the addition of [flags] to the usage line of a command when printing help or generating docs

### disableSuggestions

```ts
disableSuggestions: boolean
```

DisableSuggestions disables the suggestions based on Levenshtein distance that go along with 'unknown command' messages.

### example

```ts
example: string
```

Example is examples of how to use the command.

### fParseErrWhitelist

```ts
fParseErrWhitelist: FParseErrWhitelist
```

FParseErrWhitelist flag parse errors to be ignored

### groupID

```ts
groupID: string
```

The group id under which this subcommand is grouped in the 'help' output of its parent.

### hidden

```ts
hidden: boolean
```

Hidden defines, if this command is hidden and should NOT show up in the list of available commands.

### long

```ts
long: string
```

Long is the long message shown in the 'help ' output.

### persistentPostRun

```ts
persistentPostRun: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

PersistentPostRun: children of this command will inherit and execute after PostRun.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### persistentPostRunE

```ts
persistentPostRunE: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

PersistentPostRunE: PersistentPostRun but returns an error.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### persistentPreRun

```ts
persistentPreRun: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

The *Run functions are executed in the following order:

```
  * PersistentPreRun()
  * PreRun()
  * Run()
  * PostRun()
  * PersistentPostRun()
```

All functions get the same args, the arguments after the command name. The *PreRun and *PostRun functions will only be executed if the Run function of the current command has been declared.

PersistentPreRun: children of this command will inherit and execute.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### persistentPreRunE

```ts
persistentPreRunE: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

PersistentPreRunE: PersistentPreRun but returns an error.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### postRun

```ts
postRun: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

PostRun: run after the Run command.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### postRunE

```ts
postRunE: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

PostRunE: PostRun but returns an error.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### preRun

```ts
preRun: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

PreRun: children of this command will not inherit.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### preRunE

```ts
preRunE: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

PreRunE: PreRun but returns an error.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### run

```ts
run: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

Run: Typically the actual work function. Most commands will only implement this.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### runE

```ts
runE: ((cmd, args) => void)
```

**Type declaration**

```ts
(cmd, args): void
```

RunE: Run but returns an error.

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`

**Returns void**

### short

```ts
short: string
```

Short is the short description shown in the 'help' output.

### silenceErrors

```ts
silenceErrors: boolean
```

SilenceErrors is an option to quiet errors down stream.

### silenceUsage

```ts
silenceUsage: boolean
```

SilenceUsage is an option to silence usage when an error occurs.

### suggestFor

```ts
suggestFor: string[]
```

SuggestFor is an array of command names for which this command will be suggested - similar to aliases but only suggests.

### suggestionsMinimumDistance

```ts
suggestionsMinimumDistance: number
```

SuggestionsMinimumDistance defines minimum levenshtein distance to display suggestions. Must be > 0.

### traverseChildren

```ts
traverseChildren: boolean
```

TraverseChildren parses flags on all parents before executing child command.

### use

```ts
use: string
```

Use is the one-line usage message. Recommended syntax is as follows:

```
  [ ] identifies an optional argument. Arguments that are not enclosed in brackets are required.
  ... indicates that you can specify multiple values for the previous argument.
  |   indicates mutually exclusive information. You can use the argument to the left of the separator or the
      argument to the right of the separator. You cannot use both arguments in a single use of the command.
  { } delimits a set of mutually exclusive arguments when one of the arguments is required. If the arguments are
      optional, they are enclosed in brackets ([ ]).
```

Example: add [-F file | -D dir]... [-f format] profile

### validArgs

```ts
validArgs: Completion[]
```

ValidArgs is list of all valid non-flag arguments that are accepted in shell completions

### validArgsFunction

```ts
validArgsFunction: CompletionFunc
```

ValidArgsFunction is an optional function that provides valid non-flag arguments for shell completion. It is a dynamic version of using ValidArgs. Only one of ValidArgs and ValidArgsFunction can be used for a command.

### version

```ts
version: string
```

Version defines the version for this command. If this value is non-empty and the command does not define a "version" flag, a "version" boolean flag will be added to the command and, if specified, will print content of the "Version" variable. A shorthand "v" flag will also be added if the command does not define one.
