PITR is built into `pocketone` and **on by default**. Every app it serves starts recording history the first time it
serves; there's nothing to install.

To run without it:

```bash
pocketone serve --pitr=false
```

In your own Go build, register it before the app starts:

```go
app := pocketbase.New()
pitr.MustRegister(app, pitr.Config{})
app.Start()
```
