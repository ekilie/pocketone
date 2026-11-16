package backups

import (
	"context"
	"errors"
	"testing"
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/tests"
	restorev1 "github.com/pockethost/pocketone/ext/contracts/restore/v1"
	"github.com/pockethost/pocketone/ext/extkit"
)

func TestRestoreToPicksLatestBackupBefore(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	ctx := context.Background()
	if err := app.CreateBackup(ctx, "a.zip"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // file times have second precision on some filesystems
	mid := time.Now()
	time.Sleep(1100 * time.Millisecond)
	if err := app.CreateBackup(ctx, "b.zip"); err != nil {
		t.Fatal(err)
	}

	restored := make(chan string, 1)
	r := &restorer{app: app, restore: func(name string) { restored <- name }}
	su := extkit.Auth{Kind: "superuser"}

	p, err := r.Points(ctx, su)
	if err != nil || p.Kind != restorev1.Snapshots || len(p.Snapshots) != 2 || p.Snapshots[0].Name != "b.zip" {
		t.Fatalf("points: %+v %v", p, err)
	}
	res, err := r.RestoreTo(ctx, su, restorev1.RestoreInput{At: mid})
	if err != nil || res.Name != "a.zip" || !res.Restoring {
		t.Fatalf("restore to mid: %+v %v", res, err)
	}
	if got := <-restored; got != "a.zip" {
		t.Fatalf("restored %q", got)
	}
	if _, err := r.RestoreTo(ctx, su, restorev1.RestoreInput{At: time.Now().Add(-time.Hour)}); !errors.Is(err, restorev1.ErrNoPoint) {
		t.Fatalf("before every backup: %v", err)
	}
}
