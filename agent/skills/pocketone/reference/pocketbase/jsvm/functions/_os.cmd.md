Source: https://pocketbase.io/jsvm/functions/_os.cmd.html

# Function cmd

```ts
cmd(name, ...arg): Cmd
```

Prepares an external OS command.

Example:

```js
// prepare the command to execute
const cmd = $os.cmd('ls', '-sl')

// execute the command and return its standard output as string
const output = toString(cmd.output());
```

**Parameters**

- `name: string`
- `Rest ...arg: string[]`

**Returns Cmd**
