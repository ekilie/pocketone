Source: https://pocketbase.io/jsvm/interfaces/os.getgroups.html

# Interface getgroups

**Hierarchy**

- getgroups

```ts
getgroups(): number[]
```

Getgroups returns a list of the numeric ids of groups that the caller belongs to.

On Windows, it returns [syscall.EWINDOWS]. See the [os/user] package for a possible alternative.

**Returns number[]**
