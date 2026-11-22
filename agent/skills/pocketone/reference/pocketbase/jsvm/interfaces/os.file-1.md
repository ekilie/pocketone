Source: https://pocketbase.io/jsvm/interfaces/os.file-1.html

# Interface file

file is the real representation of *File. The extra level of indirection ensures that no clients of os can overwrite this data, which could cause the finalizer to close the wrong file descriptor.

**Hierarchy**

- file
