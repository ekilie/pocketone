Source: https://pocketbase.io/jsvm/types/excludeHooks.html

# Type alias excludeHooks<Type>

```ts
excludeHooks<Type>: {     [Property in keyof Type as Exclude<Property, `on${string}` | "cron">]: Type[Property] }
```

**Type Parameters**

- Type
