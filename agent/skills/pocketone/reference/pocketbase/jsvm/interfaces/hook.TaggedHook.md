Source: https://pocketbase.io/jsvm/interfaces/hook.TaggedHook.html

# Interface TaggedHook<T>

TaggedHook defines a proxy hook which register handlers that are triggered only if the TaggedHook.tags are empty or includes at least one of the event data tag(s).

**Type Parameters**

- T

**Hierarchy**

- [_sboPuQc](https://pocketbase.io/jsvm/types/hook._sboPuQc.html)<T>

  - TaggedHook

## Methods

### bind

```ts
bind(handler): string
```

Bind registers the provided handler to the current hooks queue.

It is similar to [Hook.Bind] with the difference that the handler function is invoked only if the event data tags satisfy h.CanTriggerOn.

**Parameters**

- `handler: hook.Handler<T>`

**Returns string**

### bindFunc

```ts
bindFunc(fn): string
```

BindFunc registers a new handler with the specified function.

It is similar to [Hook.Bind] with the difference that the handler function is invoked only if the event data tags satisfy h.CanTriggerOn.

**Parameters**

- `fn: ((e) => void)`

**Returns string**

### canTriggerOn

```ts
canTriggerOn(tagsToCheck): boolean
```

CanTriggerOn checks if the current TaggedHook can be triggered with the provided event data tags.

It returns always true if the hook doesn't have any tags.

**Parameters**

- `tagsToCheck: string[]`

**Returns boolean**

### length

```ts
length(): number
```

Length returns to total number of registered hook handlers.

**Returns number**

### trigger

```ts
trigger(event, ...oneOffHandlerFuncs): void
```

Trigger executes all registered hook handlers one by one with the specified event as an argument.

Optionally, this method allows also to register additional one off handler funcs that will be temporary appended to the handlers queue.

NB! Each hook handler must call event.Next() in order the hook chain to proceed.

**Parameters**

- `event: T`
- `Rest ...oneOffHandlerFuncs: ((_arg0) => void)[]`

**Returns void**

### unbind

```ts
unbind(...idsToRemove): void
```

Unbind removes one or many hook handler by their id.

**Parameters**

- `Rest ...idsToRemove: string[]`

**Returns void**

### unbindAll

```ts
unbindAll(): void
```

UnbindAll removes all registered handlers.

**Returns void**
