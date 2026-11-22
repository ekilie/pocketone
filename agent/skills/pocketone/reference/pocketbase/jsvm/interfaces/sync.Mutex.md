Source: https://pocketbase.io/jsvm/interfaces/sync.Mutex.html

# Interface Mutex

A Mutex is a mutual exclusion lock. The zero value for a Mutex is an unlocked mutex.

A Mutex must not be copied after first use.

In the terminology of [the Go memory model](https://go.dev/ref/mem), the n'th call to [Mutex.Unlock] “synchronizes before” the m'th call to [Mutex.Lock] for any n < m. A successful call to [Mutex.TryLock] is equivalent to a call to Lock. A failed call to TryLock does not establish any “synchronizes before” relation at all.

**Hierarchy**

- Mutex

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
