Source: https://pocketbase.io/jsvm/modules/fs.html

# Namespace fs

Package fs defines basic interfaces to a file system. A file system can be provided by the host operating system but also by other packages.

## Path Names

The interfaces in this package all operate on the same path name syntax, regardless of the host operating system.

Path names are UTF-8-encoded, unrooted, slash-separated sequences of path elements, like “x/y/z”. Path names must not contain an element that is “.” or “..” or the empty string, except for the special case that the name "." may be used for the root directory. Paths must not start or end with a slash: “/x” and “x/” are invalid.

## Testing

See the [testing/fstest] package for support with testing implementations of file systems.
