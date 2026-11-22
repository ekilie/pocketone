Source: https://pocketbase.io/jsvm/interfaces/sync.RWMutex.html

# Interface RWMutex

A RWMutex is a reader/writer mutual exclusion lock. The lock can be held by an arbitrary number of readers or a single writer. The zero value for a RWMutex is an unlocked mutex.

A RWMutex must not be copied after first use.

If any goroutine calls [RWMutex.Lock] while the lock is already held by one or more readers, concurrent calls to [RWMutex.RLock] will block until the writer has acquired (and released) the lock, to ensure that the lock eventually becomes available to the writer. Note that this prohibits recursive read-locking. A [RWMutex.RLock] cannot be upgraded into a [RWMutex.Lock], nor can a [RWMutex.Lock] be downgraded into a [RWMutex.RLock].

In the terminology of [the Go memory model](https://go.dev/ref/mem), the n'th call to [RWMutex.Unlock] “synchronizes before” the m'th call to Lock for any n < m, just as for [Mutex]. For any call to RLock, there exists an n such that the n'th call to Unlock “synchronizes before” that call to RLock, and the corresponding call to [RWMutex.RUnlock] “synchronizes before” the n+1'th call to Lock.

**Hierarchy**

- RWMutex

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
