Source: https://pocketbase.io/jsvm/interfaces/os.exit.html

# Interface exit

**Hierarchy**

- exit

```ts
exit(code): void
```

Exit causes the current program to exit with the given status code. Conventionally, code zero indicates success, non-zero an error. The program terminates immediately; deferred functions are not run.

For portability, the status code should be in the range [0, 125].

**Parameters**

- `code: number`

**Returns void**
