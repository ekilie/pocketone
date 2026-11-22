Source: https://pocketbase.io/jsvm/interfaces/forms.TestS3Filesystem.html

# Interface TestS3Filesystem

TestS3Filesystem defines a S3 filesystem connection test.

**Hierarchy**

- TestS3Filesystem

  - [TestS3FilesystemForm](https://pocketbase.io/jsvm/classes/TestS3FilesystemForm.html)

**Implemented by**

- [TestS3FilesystemForm](https://pocketbase.io/jsvm/classes/TestS3FilesystemForm.html)

## Methods

### submit

```ts
submit(): void
```

Submit validates and performs a S3 filesystem connection test.

**Returns void**

### validate

```ts
validate(): void
```

Validate makes the form validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### filesystem

```ts
filesystem: string
```

The name of the filesystem - storage or backups
