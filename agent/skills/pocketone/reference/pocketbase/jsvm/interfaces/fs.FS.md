Source: https://pocketbase.io/jsvm/interfaces/fs.FS.html

# Interface FS

An FS provides access to a hierarchical file system.

The FS interface is the minimum implementation required of the file system. A file system may implement additional interfaces, such as [ReadFileFS], to provide additional or optimized functionality.

[testing/fstest.TestFS] may be used to test implementations of an FS for correctness.

**Hierarchy**

- FS

**Indexable**

```ts
[key: string]: any
```

## Methods

### open

```ts
open(name): fs.File
```

Open opens the named file. [File.Close] must be called to release any associated resources.

When Open returns an error, it should be of type *PathError with the Op field set to "open", the Path field set to name, and the Err field describing the problem.

Open should reject attempts to open names that do not satisfy ValidPath(name), returning a *PathError with Err set to ErrInvalid or ErrNotExist.

**Parameters**

- `name: string`

**Returns fs.File**
