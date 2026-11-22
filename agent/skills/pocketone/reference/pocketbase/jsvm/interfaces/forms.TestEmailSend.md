Source: https://pocketbase.io/jsvm/interfaces/forms.TestEmailSend.html

# Interface TestEmailSend

TestEmailSend is a email template test request form.

**Hierarchy**

- TestEmailSend

  - [TestEmailSendForm](https://pocketbase.io/jsvm/classes/TestEmailSendForm.html)

**Implemented by**

- [TestEmailSendForm](https://pocketbase.io/jsvm/classes/TestEmailSendForm.html)

## Methods

### submit

```ts
submit(): void
```

Submit validates and sends a test email to the form.Email address.

**Returns void**

### validate

```ts
validate(): void
```

Validate makes the form validatable by implementing [validation.Validatable] interface.

**Returns void**

## Properties

### collection

```ts
collection: string
```

### email

```ts
email: string
```

### template

```ts
template: string
```
