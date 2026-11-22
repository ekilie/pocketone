Source: https://pocketbase.io/jsvm/classes/TestS3FilesystemForm.html

# Class TestS3FilesystemForm

TestS3Filesystem defines a S3 filesystem connection test.

**Hierarchy**

- [TestS3Filesystem](https://pocketbase.io/jsvm/interfaces/forms.TestS3Filesystem.html)

  - TestS3FilesystemForm

**Implements**

- [TestS3Filesystem](https://pocketbase.io/jsvm/interfaces/forms.TestS3Filesystem.html)

## Constructors

### constructor

```ts
new TestS3FilesystemForm(app): TestS3FilesystemForm
```

**Parameters**

- `app: excludeHooks<App>`

**Returns TestS3FilesystemForm**

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
