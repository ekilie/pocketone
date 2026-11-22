Source: https://pocketbase.io/jsvm/interfaces/apis.rateClient.html

# Interface rateClient

### Todo

evaluate swiching to sliding window with approximation counter similar to Cloudflare.

rateClient implements fixed window rate limit strategy.

**Hierarchy**

- [_sghYShV](https://pocketbase.io/jsvm/types/apis._sghYShV.html)

  - rateClient

## Methods

### lock

```ts
lock(): void
```

Lock locks m. If the lock is already in use, the calling goroutine blocks until the mutex is available.

**Returns void**

### tryLock

```ts
tryLock(): boolean
```

TryLock tries to lock m and reports whether it succeeded.

Note that while correct uses of TryLock do exist, they are rare, and use of TryLock is often a sign of a deeper problem in a particular use of mutexes.

**Returns boolean**

### unlock

```ts
unlock(): void
```

Unlock unlocks m. It is a run-time error if m is not locked on entry to Unlock.

A locked [Mutex] is not associated with a particular goroutine. It is allowed for one goroutine to lock a Mutex and then arrange for another goroutine to unlock it.

**Returns void**
