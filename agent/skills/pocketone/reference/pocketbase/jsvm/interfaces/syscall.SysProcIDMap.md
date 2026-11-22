Source: https://pocketbase.io/jsvm/interfaces/syscall.SysProcIDMap.html

# Interface SysProcIDMap

SysProcIDMap holds Container ID to Host ID mappings used for User Namespaces in Linux. See user_namespaces(7).

Note that User Namespaces are not available on a number of popular Linux versions (due to security issues), or are available but subject to AppArmor restrictions like in Ubuntu 24.04.

**Hierarchy**

- SysProcIDMap

## Properties

### containerID

```ts
containerID: number
```

### hostID

```ts
hostID: number
```

### size

```ts
size: number
```
