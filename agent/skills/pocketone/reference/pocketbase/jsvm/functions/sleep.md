Source: https://pocketbase.io/jsvm/functions/sleep.html

# Function sleep

```ts
sleep(milliseconds): void
```

sleep pauses the current goroutine for at least the specified user duration (in ms). A zero or negative duration returns immediately.

Example:

```js
sleep(250) // sleeps for 250ms
```

**Parameters**

- `milliseconds: number`

**Returns void**
