Source: https://pocketbase.io/jsvm/functions/_app.restart.html

# Function restart

```ts
restart(): void
```

Restart restarts (aka. replaces) the current running application process.

NB! It relies on execve which is supported only on UNIX based systems.

**Returns void**
