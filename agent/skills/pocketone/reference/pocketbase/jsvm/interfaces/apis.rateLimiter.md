Source: https://pocketbase.io/jsvm/interfaces/apis.rateLimiter.html

# Interface rateLimiter

**Hierarchy**

- [_sSOaUvo](https://pocketbase.io/jsvm/types/apis._sSOaUvo.html)

  - rateLimiter

## Methods

### lock

```ts
lock(): void
```

Lock locks rw for writing. If the lock is already locked for reading or writing, Lock blocks until the lock is available.

**Returns void**

### rLock

```ts
rLock(): void
```

RLock locks rw for reading.

It should not be used for recursive read locking; a blocked Lock call excludes new readers from acquiring the lock. See the documentation on the [RWMutex] type.

**Returns void**

### rLocker

```ts
rLocker(): Locker
```

RLocker returns a [Locker] interface that implements the [Locker.Lock] and [Locker.Unlock] methods by calling rw.RLock and rw.RUnlock.

**Returns Locker**

### rUnlock

```ts
rUnlock(): void
```

RUnlock undoes a single [RWMutex.RLock] call; it does not affect other simultaneous readers. It is a run-time error if rw is not locked for reading on entry to RUnlock.

**Returns void**

### tryLock

```ts
tryLock(): boolean
```

TryLock tries to lock rw for writing and reports whether it succeeded.

Note that while correct uses of TryLock do exist, they are rare, and use of TryLock is often a sign of a deeper problem in a particular use of mutexes.

**Returns boolean**

### tryRLock

```ts
tryRLock(): boolean
```

TryRLock tries to lock rw for reading and reports whether it succeeded.

Note that while correct uses of TryRLock do exist, they are rare, and use of TryRLock is often a sign of a deeper problem in a particular use of mutexes.

**Returns boolean**

### unlock

```ts
unlock(): void
```

Unlock unlocks rw for writing. It is a run-time error if rw is not locked for writing on entry to Unlock.

As with Mutexes, a locked [RWMutex] is not associated with a particular goroutine. One goroutine may [RWMutex.RLock] ([RWMutex.Lock]) a RWMutex and then arrange for another goroutine to [RWMutex.RUnlock] ([RWMutex.Unlock]) it.

**Returns void**
