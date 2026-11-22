Source: https://pocketbase.io/jsvm/modules/cron.html

# Namespace cron

Package cron implements a crontab-like service to execute and schedule repeative tasks/jobs.

Example:

```
 c := cron.New()
 c.MustAdd("dailyReport", "0 0 * * *", func() { ... })
 c.Start()
```
