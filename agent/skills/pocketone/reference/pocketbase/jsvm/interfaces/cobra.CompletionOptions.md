Source: https://pocketbase.io/jsvm/interfaces/cobra.CompletionOptions.html

# Interface CompletionOptions

CompletionOptions are the options to control shell completion

**Hierarchy**

- CompletionOptions

## Methods

### setDefaultShellCompDirective

```ts
setDefaultShellCompDirective(directive): void
```

**Parameters**

- `directive: ShellCompDirective`

**Returns void**

## Properties

### Optional defaultShellCompDirective

```ts
defaultShellCompDirective?: ShellCompDirective
```

DefaultShellCompDirective sets the ShellCompDirective that is returned if no special directive can be determined

### disableDefaultCmd

```ts
disableDefaultCmd: boolean
```

DisableDefaultCmd prevents Cobra from creating a default 'completion' command

### disableDescriptions

```ts
disableDescriptions: boolean
```

DisableDescriptions turns off all completion descriptions for shells that support them

### disableNoDescFlag

```ts
disableNoDescFlag: boolean
```

DisableNoDescFlag prevents Cobra from creating the '--no-descriptions' flag for shells that support completion descriptions

### hiddenDefaultCmd

```ts
hiddenDefaultCmd: boolean
```

HiddenDefaultCmd makes the default 'completion' command hidden
