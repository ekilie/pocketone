Source: https://pocketbase.io/jsvm/functions/_app.execute.html

# Function execute

```ts
execute(): void
```

Execute initializes the application (if not already) and executes the pb.RootCmd with graceful shutdown support.

This method differs from pb.Start() by not registering the default system commands!

**Returns void**
