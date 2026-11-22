Source: https://pocketbase.io/jsvm/interfaces/cobra.CompletionFunc.html

# Interface CompletionFunc

CompletionFunc is a function that provides completion results.

**Hierarchy**

- CompletionFunc

```ts
CompletionFunc(cmd, args, toComplete): [Completion[], ShellCompDirective]
```

**Parameters**

- `cmd: cobra.Command`
- `args: string[]`
- `toComplete: string`

**Returns [Completion[], ShellCompDirective]**
