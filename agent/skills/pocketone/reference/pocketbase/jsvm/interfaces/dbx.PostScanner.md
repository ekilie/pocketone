Source: https://pocketbase.io/jsvm/interfaces/dbx.PostScanner.html

# Interface PostScanner

PostScanner is an optional interface used by ScanStruct.

**Hierarchy**

- PostScanner

**Indexable**

```ts
[key: string]: any
```

## Methods

### postScan

```ts
postScan(): void
```

PostScan executes right after the struct has been populated with the DB values, allowing you to further normalize or validate the loaded data.

**Returns void**
