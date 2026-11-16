// A PocketOne app with the standard set and one extra extension: run it with `go run . serve`.
package main

import (
	"log"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/pockethost/pocketone/ext/mailtrap"
	"github.com/pockethost/pocketone/ext/pitr"
	"github.com/pockethost/pocketone/ext/standard"
)

func main() {
	set := standard.New()
	set.PITR = pitr.New(pitr.Config{Retention: 7 * 24 * time.Hour}) // reconfigure a member
	set.Add(mailtrap.New())                                         // add your own

	app := set.NewApp(pocketbase.Config{}, jsvm.Config{HooksDir: "pb_hooks"})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
