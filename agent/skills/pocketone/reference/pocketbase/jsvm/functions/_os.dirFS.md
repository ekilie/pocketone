Source: https://pocketbase.io/jsvm/functions/_os.dirFS.html

# Function dirFS

```ts
dirFS(dir): FS
```

DirFS returns a file system (an fs.FS) for the tree of files rooted at the directory dir.

Note that DirFS("/prefix") only guarantees that the Open calls it makes to the operating system will begin with "/prefix": DirFS("/prefix").Open("file") is the same as os.Open("/prefix/file"). So if /prefix/file is a symbolic link pointing outside the /prefix tree, then using DirFS does not stop the access any more than using os.Open does. Additionally, the root of the fs.FS returned for a relative path, DirFS("prefix"), will be affected by later calls to Chdir. DirFS is therefore not a general substitute for a chroot-style security mechanism when the directory tree contains arbitrary content.

Use [Root.FS] to obtain a fs.FS that prevents escapes from the tree via symbolic links.

The directory dir must not be "".

The result implements [io/fs.StatFS], [io/fs.ReadFileFS], [io/fs.ReadDirFS], and [io/fs.ReadLinkFS].

**Parameters**

- `dir: string`

**Returns FS**
